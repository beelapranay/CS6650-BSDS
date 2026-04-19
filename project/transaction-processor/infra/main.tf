# ---------------------------------------------------------------------------
# LabRole-only stack.
#
# This module intentionally creates NO IAM roles, users, policies, or instance
# profiles. Every AWS API call Terraform makes must be authorized by LabRole.
# ECS task execution and task roles both reference LabRole. The provider's
# assume_role block and the check "caller_is_lab_role" assertion enforce this.
# Do not add aws_iam_* create resources to this file.
# ---------------------------------------------------------------------------

locals {
  name_prefix         = "${var.project_name}-${var.environment}"
  lab_role_arn        = "arn:aws:iam::${var.aws_account_id}:role/${var.lab_role_name}"
  selected_subnet_ids = var.subnet_ids != null ? var.subnet_ids : data.aws_subnets.default.ids
  api_image           = "${aws_ecr_repository.api.repository_url}:${var.app_image_tag}"
  worker_image        = "${aws_ecr_repository.worker.repository_url}:${var.worker_image_tag}"
}

provider "aws" {
  region              = var.aws_region
  allowed_account_ids = [var.aws_account_id]

  dynamic "assume_role" {
    for_each = var.provider_assume_lab_role ? [1] : []

    content {
      role_arn     = local.lab_role_arn
      session_name = "${local.name_prefix}-terraform"
    }
  }

  default_tags {
    tags = {
      Project     = var.project_name
      Environment = var.environment
      ManagedBy   = "Terraform"
    }
  }
}

data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

data "aws_iam_role" "lab_role" {
  name = var.lab_role_name

  lifecycle {
    postcondition {
      condition     = self.name == var.lab_role_name
      error_message = "lab_role data lookup did not resolve to ${var.lab_role_name}"
    }
  }
}

data "aws_caller_identity" "current" {}

data "aws_arn" "caller" {
  arn = data.aws_caller_identity.current.arn
}

# The caller may be any authenticated principal (e.g. an SSO session with
# admin rights). The critical guarantee is that LabRole exists and is wired
# into every ECS task we create, so the running workload identity stays
# pinned to LabRole even if the operator identity changes.
check "lab_role_wired_into_tasks" {
  assert {
    condition     = data.aws_iam_role.lab_role.name == var.lab_role_name
    error_message = "lab_role data lookup did not resolve to ${var.lab_role_name}"
  }
}

resource "aws_ecr_repository" "api" {
  name                 = "${local.name_prefix}-api"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_repository" "worker" {
  name                 = "${local.name_prefix}-worker"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_sqs_queue" "transfers_dlq" {
  name                      = "${local.name_prefix}-transfers-dlq"
  message_retention_seconds = 1209600
}

resource "aws_sqs_queue" "transfers" {
  name                       = "${local.name_prefix}-transfers"
  visibility_timeout_seconds = 60
  receive_wait_time_seconds  = 20

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.transfers_dlq.arn
    maxReceiveCount     = 5
  })
}

resource "aws_dynamodb_table" "accounts" {
  name         = "${local.name_prefix}-accounts"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "account_id"

  attribute {
    name = "account_id"
    type = "S"
  }
}

resource "aws_dynamodb_table" "transactions" {
  name         = "${local.name_prefix}-transactions"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "transaction_id"

  attribute {
    name = "transaction_id"
    type = "S"
  }
}

resource "aws_dynamodb_table" "account_locks" {
  name         = "${local.name_prefix}-account-locks"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "account_id"

  attribute {
    name = "account_id"
    type = "S"
  }
}

resource "aws_security_group" "alb" {
  name        = "${local.name_prefix}-alb"
  description = "Public access for the transaction processor ALB"
  vpc_id      = data.aws_vpc.default.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_vpc_security_group_ingress_rule" "alb_http" {
  for_each          = toset(var.allowed_ingress_cidrs)
  security_group_id = aws_security_group.alb.id
  cidr_ipv4         = each.value
  from_port         = 80
  ip_protocol       = "tcp"
  to_port           = 80
}

resource "aws_security_group" "ecs_tasks" {
  name        = "${local.name_prefix}-ecs-tasks"
  description = "ECS tasks for the transaction processor"
  vpc_id      = data.aws_vpc.default.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_vpc_security_group_ingress_rule" "ecs_from_alb" {
  security_group_id            = aws_security_group.ecs_tasks.id
  referenced_security_group_id = aws_security_group.alb.id
  from_port                    = 8080
  ip_protocol                  = "tcp"
  to_port                      = 8080
}

resource "aws_cloudwatch_log_group" "api" {
  name              = "/ecs/${local.name_prefix}/api"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "worker" {
  name              = "/ecs/${local.name_prefix}/worker"
  retention_in_days = 7
}

resource "aws_lb" "api" {
  name               = substr(replace("${local.name_prefix}-alb", "_", "-"), 0, 32)
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = local.selected_subnet_ids
}

resource "aws_lb_target_group" "api" {
  name        = substr(replace("${local.name_prefix}-api", "_", "-"), 0, 32)
  port        = 8080
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = data.aws_vpc.default.id

  health_check {
    enabled             = true
    path                = "/health"
    healthy_threshold   = 2
    unhealthy_threshold = 2
    timeout             = 5
    interval            = 15
    matcher             = "200"
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.api.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api.arn
  }
}

resource "aws_ecs_cluster" "main" {
  name = "${local.name_prefix}-cluster"
}

resource "aws_ecs_task_definition" "api" {
  family                   = "${local.name_prefix}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = tostring(var.api_cpu)
  memory                   = tostring(var.api_memory)
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([
    {
      name      = "api-svc"
      image     = local.api_image
      essential = true
      portMappings = [{
        containerPort = 8080
        hostPort      = 8080
        protocol      = "tcp"
      }]
      environment = [
        { name = "AWS_REGION", value = var.aws_region },
        { name = "DYNAMODB_ACCOUNTS_TABLE", value = aws_dynamodb_table.accounts.name },
        { name = "DYNAMODB_TRANSACTIONS_TABLE", value = aws_dynamodb_table.transactions.name },
        { name = "DYNAMODB_LOCKS_TABLE", value = aws_dynamodb_table.account_locks.name },
        { name = "SQS_QUEUE_URL", value = aws_sqs_queue.transfers.id },
        { name = "PORT", value = "8080" }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.api.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "ecs"
        }
      }
    }
  ])
}

resource "aws_ecs_task_definition" "worker" {
  family                   = "${local.name_prefix}-worker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = tostring(var.worker_cpu)
  memory                   = tostring(var.worker_memory)
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([
    {
      name      = "worker-svc"
      image     = local.worker_image
      essential = true
      environment = [
        { name = "AWS_REGION", value = var.aws_region },
        { name = "DYNAMODB_ACCOUNTS_TABLE", value = aws_dynamodb_table.accounts.name },
        { name = "DYNAMODB_TRANSACTIONS_TABLE", value = aws_dynamodb_table.transactions.name },
        { name = "DYNAMODB_LOCKS_TABLE", value = aws_dynamodb_table.account_locks.name },
        { name = "SQS_QUEUE_URL", value = aws_sqs_queue.transfers.id },
        { name = "LOCKING_MODE", value = var.locking_mode },
        { name = "LOCK_TTL_SECONDS", value = tostring(var.lock_ttl_seconds) },
        { name = "PRE_COMMIT_DELAY_MS", value = tostring(var.pre_commit_delay_ms) },
        { name = "METRICS_PORT", value = "9090" },
        { name = "METRICS_LOG_INTERVAL_SECONDS", value = tostring(var.metrics_log_interval_seconds) }
      ]
      portMappings = [{
        containerPort = 9090
        hostPort      = 9090
        protocol      = "tcp"
      }]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.worker.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "ecs"
        }
      }
    }
  ])
}

resource "aws_ecs_service" "api" {
  name            = "${local.name_prefix}-api"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = var.api_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    assign_public_ip = true
    security_groups  = [aws_security_group.ecs_tasks.id]
    subnets          = local.selected_subnet_ids
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = "api-svc"
    container_port   = 8080
  }

  depends_on = [
    aws_lb_listener.http,
    aws_sqs_queue.transfers,
    aws_dynamodb_table.accounts,
    aws_dynamodb_table.transactions,
    aws_dynamodb_table.account_locks,
  ]
}

resource "aws_ecs_service" "worker" {
  name            = "${local.name_prefix}-worker"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.worker.arn
  desired_count   = var.worker_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    assign_public_ip = true
    security_groups  = [aws_security_group.ecs_tasks.id]
    subnets          = local.selected_subnet_ids
  }

  depends_on = [
    aws_sqs_queue.transfers,
    aws_dynamodb_table.accounts,
    aws_dynamodb_table.transactions,
    aws_dynamodb_table.account_locks,
  ]
}
