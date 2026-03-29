resource "aws_db_subnet_group" "this" {
  name        = "${var.identifier}-subnets"
  description = "Private subnets for the Homework 8 MySQL instance"
  subnet_ids  = var.private_subnet_ids

  tags = merge(var.tags, {
    Name = "${var.identifier}-subnets"
  })
}

resource "aws_security_group" "this" {
  name        = "${var.identifier}-sg"
  description = "Allow MySQL access from ECS tasks only"
  vpc_id      = var.vpc_id

  ingress {
    description     = "MySQL from ECS tasks"
    from_port       = 3306
    to_port         = 3306
    protocol        = "tcp"
    security_groups = [var.ecs_task_security_group_id]
  }

  egress {
    description = "Allow outbound traffic for managed RDS operations"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.identifier}-sg"
  })
}

resource "aws_db_parameter_group" "this" {
  name        = "${var.identifier}-mysql8"
  family      = "mysql8.0"
  description = "Parameter group for Homework 8 MySQL 8.0"

  parameter {
    name  = "character_set_server"
    value = "utf8mb4"
  }

  parameter {
    name  = "collation_server"
    value = "utf8mb4_unicode_ci"
  }

  tags = merge(var.tags, {
    Name = "${var.identifier}-mysql8"
  })
}

resource "aws_db_instance" "this" {
  identifier                   = var.identifier
  engine                       = "mysql"
  engine_version               = var.engine_version
  instance_class               = var.instance_class
  allocated_storage            = var.allocated_storage
  max_allocated_storage        = var.max_allocated_storage
  storage_type                 = "gp3"
  storage_encrypted            = true
  db_name                      = var.db_name
  username                     = var.username
  password                     = var.password
  port                         = 3306
  db_subnet_group_name         = aws_db_subnet_group.this.name
  vpc_security_group_ids       = [aws_security_group.this.id]
  parameter_group_name         = aws_db_parameter_group.this.name
  publicly_accessible          = false
  deletion_protection          = false
  skip_final_snapshot          = true
  backup_retention_period      = 0
  auto_minor_version_upgrade   = true
  apply_immediately            = true
  multi_az                     = false
  performance_insights_enabled = false
  monitoring_interval          = 0

  tags = merge(var.tags, {
    Name = var.identifier
  })
}
