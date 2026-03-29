variable "name_prefix" {
  description = "Name prefix for ALB resources."
  type        = string
}

variable "vpc_id" {
  description = "VPC ID for the ALB."
  type        = string
}

variable "public_subnet_ids" {
  description = "Public subnet IDs for the ALB."
  type        = list(string)
}

variable "container_port" {
  description = "Backend container port."
  type        = number
}

variable "health_check_path" {
  description = "Health check path for the target group."
  type        = string
}

variable "tags" {
  description = "Tags applied to ALB resources."
  type        = map(string)
}
