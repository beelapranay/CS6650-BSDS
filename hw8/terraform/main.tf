locals {
  container_image = "${module.ecr.repository_url}:latest"
  db_env_vars = {
    AWS_REGION            = var.aws_region
    DB_TYPE               = var.store_backend
    DB_HOST               = module.rds_mysql.endpoint
    DB_PORT               = tostring(module.rds_mysql.port)
    DB_NAME               = module.rds_mysql.db_name
    DB_USER               = var.db_username
    DB_PASSWORD           = var.db_password
    DB_MAX_OPEN_CONNS     = "25"
    DB_MAX_IDLE_CONNS     = "10"
    DB_CONN_MAX_LIFETIME  = "5m"
    DB_CONN_MAX_IDLE_TIME = "2m"
    DYNAMODB_TABLE_NAME   = module.dynamodb.table_name
  }
}

data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

module "network" {
  source = "./modules/network"

  name_prefix = var.name_prefix
  vpc_cidr    = var.vpc_cidr
  tags        = var.tags
}

module "ecr" {
  source = "./modules/ecr"

  repository_name = "${var.name_prefix}-cart-api"
  tags            = var.tags
}

module "logging" {
  source = "./modules/logging"

  log_group_name    = "/ecs/${var.name_prefix}-cart-api"
  retention_in_days = var.log_retention_days
  tags              = var.tags
}

module "alb" {
  source = "./modules/alb"

  name_prefix       = var.name_prefix
  vpc_id            = module.network.vpc_id
  public_subnet_ids = module.network.public_subnet_ids
  container_port    = var.container_port
  health_check_path = var.health_check_path
  tags              = var.tags
}

resource "aws_security_group" "ecs_tasks" {
  name        = "${var.name_prefix}-ecs-sg"
  description = "Allow app traffic from the ALB only"
  vpc_id      = module.network.vpc_id

  ingress {
    description     = "App traffic from ALB"
    from_port       = var.container_port
    to_port         = var.container_port
    protocol        = "tcp"
    security_groups = [module.alb.security_group_id]
  }

  egress {
    description = "Allow outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.name_prefix}-ecs-sg"
  })
}

resource "null_resource" "push_image" {
  triggers = {
    repository_url = module.ecr.repository_url
    dockerfile_sha = filesha256("${path.module}/../src/Dockerfile")
    source_sha     = filesha256("${path.module}/../src/main.go")
    gomod_sha      = filesha256("${path.module}/../src/go.mod")
    gosum_sha      = filesha256("${path.module}/../src/go.sum")
  }

  provisioner "local-exec" {
    command = <<EOT
set -e
aws ecr get-login-password --region ${var.aws_region} | docker login --username AWS --password-stdin ${module.ecr.repository_url}
docker build --platform linux/amd64 -t ${module.ecr.repository_url}:latest ${path.module}/../src
docker push ${module.ecr.repository_url}:latest
EOT
  }
}

module "ecs" {
  source = "./modules/ecs"

  name_prefix           = var.name_prefix
  region                = var.aws_region
  image                 = local.container_image
  container_port        = var.container_port
  cpu                   = var.ecs_cpu
  memory                = var.ecs_memory
  desired_count         = var.ecs_desired_count
  private_subnet_ids    = module.network.private_app_subnet_ids
  security_group_ids    = [aws_security_group.ecs_tasks.id]
  target_group_arn      = module.alb.target_group_arn
  log_group_name        = module.logging.log_group_name
  environment_variables = local.db_env_vars
  execution_role_arn    = data.aws_iam_role.lab_role.arn
  task_role_arn         = data.aws_iam_role.lab_role.arn
  tags                  = var.tags

  depends_on = [null_resource.push_image]
}

module "rds_mysql" {
  source = "./modules/rds_mysql"

  identifier                 = "${var.name_prefix}-mysql"
  db_name                    = var.db_name
  username                   = var.db_username
  password                   = var.db_password
  vpc_id                     = module.network.vpc_id
  private_subnet_ids         = module.network.private_db_subnet_ids
  ecs_task_security_group_id = aws_security_group.ecs_tasks.id
  engine_version             = var.mysql_engine_version
  instance_class             = var.db_instance_class
  allocated_storage          = var.db_allocated_storage
  max_allocated_storage      = var.db_max_allocated_storage
  tags                       = var.tags
}

module "dynamodb" {
  source = "./modules/dynamodb"

  table_name = var.dynamodb_table_name
  tags       = var.tags
}
