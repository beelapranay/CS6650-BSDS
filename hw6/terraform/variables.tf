# Region to deploy into
variable "aws_region" {
  type    = string
  default = "us-west-2"
}

# ECR & ECS settings
variable "ecr_repository_name" {
  type    = string
  default = "ecr_service"
}

variable "service_name" {
  type    = string
  default = "CS6650L2"
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "ecs_count" {
  type    = number
  default = 1
}

# Auto Scaling defaults (Part 3)
variable "autoscaling_min" {
  type    = number
  default = 2
}

variable "autoscaling_max" {
  type    = number
  default = 6
}

variable "autoscaling_cpu_target" {
  type    = number
  default = 50
}

variable "autoscaling_scale_out_cooldown" {
  type    = number
  default = 60
}

variable "autoscaling_scale_in_cooldown" {
  type    = number
  default = 60
}

# How long to keep logs
variable "log_retention_days" {
  type    = number
  default = 7
}
