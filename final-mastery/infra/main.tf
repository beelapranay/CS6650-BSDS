locals {
  name_prefix        = "${var.project_name}-${var.environment}"
  lab_role_arn       = "arn:aws:iam::${var.aws_account_id}:role/${var.lab_role_name}"
  bucket_name        = coalesce(var.bucket_name, "${local.name_prefix}-${random_string.bucket_suffix.result}")
  selected_subnet_ids = (
    var.subnet_ids != null ? var.subnet_ids : (
      var.subnet_id != null ? [var.subnet_id] : data.aws_subnets.default.ids
    )
  )
  api_image    = "${aws_ecr_repository.api.repository_url}:${var.app_image_tag}"
  worker_image = "${aws_ecr_repository.worker.repository_url}:${var.worker_image_tag}"

  common_environment = [
    {
      name  = "ALBUM_STORE_BACKEND"
      value = "aws"
    },
    {
      name  = "AWS_REGION"
      value = var.aws_region
    },
    {
      name  = "ALBUMS_TABLE_NAME"
      value = aws_dynamodb_table.albums.name
    },
    {
      name  = "PHOTOS_TABLE_NAME"
      value = aws_dynamodb_table.photos.name
    },
    {
      name  = "PHOTO_COUNTERS_TABLE_NAME"
      value = aws_dynamodb_table.photo_counters.name
    },
    {
      name  = "PHOTO_BUCKET_NAME"
      value = aws_s3_bucket.photos.bucket
    },
    {
      name  = "PHOTO_QUEUE_URL"
      value = aws_sqs_queue.photo_jobs.id
    },
    {
      name  = "S3_PRESIGN_TTL_SECONDS"
      value = tostring(var.s3_presign_ttl_seconds)
    },
    {
      name  = "MAX_UPLOAD_BYTES"
      value = tostring(var.max_upload_bytes)
    },
    {
      name  = "LOG_LEVEL"
      value = var.log_level
    },
    {
      name  = "GUNICORN_WORKERS"
      value = "4"
    },
    {
      name  = "GUNICORN_THREADS"
      value = "16"
    },
    {
      name  = "WORKER_THREADS"
      value = "20"
    }
  ]
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
      Role        = var.lab_role_name
    }
  }
}

resource "random_string" "bucket_suffix" {
  length  = 8
  upper   = false
  special = false
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

resource "aws_s3_bucket" "photos" {
  bucket        = local.bucket_name
  force_destroy = true
}

resource "aws_s3_bucket_versioning" "photos" {
  bucket = aws_s3_bucket.photos.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "photos" {
  bucket = aws_s3_bucket.photos.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "photos" {
  bucket = aws_s3_bucket.photos.id

  block_public_acls       = false
  block_public_policy     = false
  ignore_public_acls      = false
  restrict_public_buckets = false
}

resource "aws_s3_bucket_policy" "photos_public_read" {
  bucket = aws_s3_bucket.photos.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "PublicReadGetObject"
        Effect    = "Allow"
        Principal = "*"
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.photos.arn}/*"
      }
    ]
  })

  depends_on = [aws_s3_bucket_public_access_block.photos]
}

resource "aws_sqs_queue" "photo_dlq" {
  name                      = "${local.name_prefix}-photo-dlq"
  message_retention_seconds = 1209600
}

resource "aws_sqs_queue" "photo_jobs" {
  name                       = "${local.name_prefix}-photo-jobs"
  visibility_timeout_seconds = 120
  message_retention_seconds  = 345600
  receive_wait_time_seconds  = 20

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.photo_dlq.arn
    maxReceiveCount     = 5
  })
}

resource "aws_dynamodb_table" "albums" {
  name         = "${local.name_prefix}-albums"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"

  attribute {
    name = "album_id"
    type = "S"
  }
}

resource "aws_dynamodb_table" "photos" {
  name         = "${local.name_prefix}-photos"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "photo_id"

  attribute {
    name = "photo_id"
    type = "S"
  }
}

resource "aws_dynamodb_table" "photo_counters" {
  name         = "${local.name_prefix}-photo-counters"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"

  attribute {
    name = "album_id"
    type = "S"
  }
}

resource "aws_security_group" "alb" {
  name        = "${local.name_prefix}-alb"
  description = "Public ALB access for the Album Store API"
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
  description = "Access for ECS tasks"
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
  from_port                    = 8000
  ip_protocol                  = "tcp"
  to_port                      = 8000
}

resource "aws_lb" "api" {
  name               = substr("${local.name_prefix}-alb", 0, 32)
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = local.selected_subnet_ids

  lifecycle {
    precondition {
      condition     = length(local.selected_subnet_ids) >= 2
      error_message = "ECS with a public ALB requires at least two subnets in different availability zones. Set subnet_ids accordingly."
    }
  }
}

resource "aws_lb_target_group" "api" {
  name        = substr("${local.name_prefix}-api-tg", 0, 32)
  port        = 8000
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = data.aws_vpc.default.id

  health_check {
    enabled             = true
    path                = "/health"
    protocol            = "HTTP"
    matcher             = "200"
    healthy_threshold   = 2
    unhealthy_threshold = 5
    timeout             = 5
    interval            = 30
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
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = tostring(var.api_task_cpu)
  memory                   = tostring(var.api_task_memory)
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  runtime_platform {
    cpu_architecture        = "ARM64"
    operating_system_family = "LINUX"
  }

  container_definitions = jsonencode([
    {
      name      = "api"
      image     = local.api_image
      essential = true
      portMappings = [
        {
          containerPort = 8000
          hostPort      = 8000
          protocol      = "tcp"
        }
      ]
      environment = local.common_environment
    }
  ])
}

resource "aws_ecs_task_definition" "worker" {
  family                   = "${local.name_prefix}-worker"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = tostring(var.worker_task_cpu)
  memory                   = tostring(var.worker_task_memory)
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  runtime_platform {
    cpu_architecture        = "ARM64"
    operating_system_family = "LINUX"
  }

  container_definitions = jsonencode([
    {
      name      = "worker"
      image     = local.worker_image
      essential = true
      command   = ["/app/album-store-worker"]
      environment = local.common_environment
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
    subnets          = local.selected_subnet_ids
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = "api"
    container_port   = 8000
  }

  health_check_grace_period_seconds = 30

  depends_on = [
    aws_lb_listener.http,
    aws_ecr_repository.api,
    aws_s3_bucket.photos,
    aws_sqs_queue.photo_jobs,
    aws_dynamodb_table.albums,
    aws_dynamodb_table.photos,
    aws_dynamodb_table.photo_counters,
  ]
}

resource "aws_ecs_service" "worker" {
  name            = "${local.name_prefix}-worker"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.worker.arn
  desired_count   = var.worker_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = local.selected_subnet_ids
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = true
  }

  depends_on = [
    aws_ecr_repository.worker,
    aws_s3_bucket.photos,
    aws_sqs_queue.photo_jobs,
    aws_dynamodb_table.albums,
    aws_dynamodb_table.photos,
    aws_dynamodb_table.photo_counters,
  ]
}
