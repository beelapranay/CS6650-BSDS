variable "name_prefix" {
  description = "Name prefix for ECS resources."
  type        = string
}

variable "region" {
  description = "AWS region for logging configuration."
  type        = string
}

variable "image" {
  description = "Container image for the ECS task."
  type        = string
}

variable "container_port" {
  description = "Container port exposed by the app."
  type        = number
}

variable "cpu" {
  description = "Fargate CPU units."
  type        = number
}

variable "memory" {
  description = "Fargate memory in MiB."
  type        = number
}

variable "desired_count" {
  description = "Desired ECS service count."
  type        = number
}

variable "private_subnet_ids" {
  description = "Private subnet IDs for the ECS service."
  type        = list(string)
}

variable "security_group_ids" {
  description = "Security group IDs attached to the ECS tasks."
  type        = list(string)
}

variable "target_group_arn" {
  description = "ALB target group ARN."
  type        = string
}

variable "log_group_name" {
  description = "CloudWatch log group name."
  type        = string
}

variable "execution_role_arn" {
  description = "IAM role ARN used by ECS for image pulls and logs."
  type        = string
}

variable "task_role_arn" {
  description = "IAM role ARN used by the running task."
  type        = string
}

variable "environment_variables" {
  description = "Environment variables passed to the container."
  type        = map(string)
  default     = {}
}

variable "tags" {
  description = "Tags applied to ECS resources."
  type        = map(string)
}
