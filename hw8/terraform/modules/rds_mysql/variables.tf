variable "identifier" {
  description = "RDS instance identifier."
  type        = string
}

variable "db_name" {
  description = "Initial database name."
  type        = string
}

variable "username" {
  description = "Master username."
  type        = string
}

variable "password" {
  description = "Master password."
  type        = string
  sensitive   = true
}

variable "vpc_id" {
  description = "VPC ID where the RDS instance lives."
  type        = string
}

variable "private_subnet_ids" {
  description = "Private subnet IDs for the DB subnet group."
  type        = list(string)
}

variable "ecs_task_security_group_id" {
  description = "Security group ID for the ECS tasks that should reach MySQL."
  type        = string
}

variable "engine_version" {
  description = "MySQL engine version."
  type        = string
}

variable "instance_class" {
  description = "RDS instance class."
  type        = string
}

variable "allocated_storage" {
  description = "Initial storage allocation in GB."
  type        = number
}

variable "max_allocated_storage" {
  description = "Maximum autoscaled storage in GB."
  type        = number
}

variable "tags" {
  description = "Tags applied to all resources."
  type        = map(string)
}
