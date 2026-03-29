variable "aws_region" {
  description = "AWS region for the Homework 8 deployment."
  type        = string
  default     = "us-west-2"
}

variable "name_prefix" {
  description = "Name prefix applied across Terraform resources."
  type        = string
  default     = "hw8"
}

variable "vpc_cidr" {
  description = "CIDR block for the Homework 8 VPC."
  type        = string
  default     = "10.0.0.0/16"
}

variable "container_port" {
  description = "Container port exposed by the ECS task."
  type        = number
  default     = 8080
}

variable "health_check_path" {
  description = "ALB health check path."
  type        = string
  default     = "/health"
}

variable "ecs_cpu" {
  description = "Fargate CPU units."
  type        = number
  default     = 256
}

variable "ecs_memory" {
  description = "Fargate memory in MiB."
  type        = number
  default     = 512
}

variable "ecs_desired_count" {
  description = "Desired ECS task count."
  type        = number
  default     = 1
}

variable "log_retention_days" {
  description = "CloudWatch Logs retention for the ECS service."
  type        = number
  default     = 7
}

variable "db_name" {
  description = "Initial database name for the shopping cart service."
  type        = string
  default     = "shopping_cart"
}

variable "db_username" {
  description = "Master username for the RDS MySQL instance."
  type        = string
  default     = "cart_admin"
}

variable "db_password" {
  description = "Master password for the RDS MySQL instance."
  type        = string
  sensitive   = true
}

variable "mysql_engine_version" {
  description = "MySQL engine version. Keep this on the 8.0 major line for the assignment."
  type        = string
  default     = "8.0"
}

variable "db_instance_class" {
  description = "RDS instance class for the assignment."
  type        = string
  default     = "db.t3.micro"
}

variable "db_allocated_storage" {
  description = "Initial RDS storage size in GB."
  type        = number
  default     = 20
}

variable "db_max_allocated_storage" {
  description = "Maximum autoscaled RDS storage in GB."
  type        = number
  default     = 100
}

variable "dynamodb_table_name" {
  description = "DynamoDB table name for the shopping cart service."
  type        = string
  default     = "hw8-shopping-carts"
}

variable "store_backend" {
  description = "Active backend for the cart API. Supported values: mysql, dynamodb."
  type        = string
  default     = "mysql"
}

variable "tags" {
  description = "Common tags applied to Homework 8 resources."
  type        = map(string)
  default = {
    Project     = "hw8"
    Environment = "assignment"
    ManagedBy   = "terraform"
  }
}
