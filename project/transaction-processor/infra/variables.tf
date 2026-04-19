variable "aws_region" {
  description = "AWS region for all resources."
  type        = string
  default     = "us-west-2"
}

variable "aws_account_id" {
  description = "AWS account ID used for safety checks and LabRole ARN construction."
  type        = string
}

variable "lab_role_name" {
  description = "Only IAM role allowed for provider assume-role and ECS task execution."
  type        = string
  default     = "LabRole"
}

variable "provider_assume_lab_role" {
  description = "Whether Terraform should assume LabRole before creating resources. Leave false if the caller is already authenticated as LabRole (standard AWS Academy setup)."
  type        = bool
  default     = false
}

variable "project_name" {
  description = "Project slug used in resource names."
  type        = string
  default     = "transaction-processor"
}

variable "environment" {
  description = "Environment suffix for resource naming."
  type        = string
  default     = "dev"
}

variable "subnet_ids" {
  description = "Optional subnet IDs for ALB and ECS. Defaults to the default VPC subnets."
  type        = list(string)
  default     = null
}

variable "allowed_ingress_cidrs" {
  description = "CIDRs allowed to reach the public ALB."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "api_desired_count" {
  description = "Desired number of API tasks."
  type        = number
  default     = 1
}

variable "worker_desired_count" {
  description = "Desired number of worker tasks."
  type        = number
  default     = 1
}

variable "api_cpu" {
  description = "Fargate CPU units for the API task."
  type        = number
  default     = 256
}

variable "api_memory" {
  description = "Fargate memory in MiB for the API task."
  type        = number
  default     = 512
}

variable "worker_cpu" {
  description = "Fargate CPU units for the worker task."
  type        = number
  default     = 256
}

variable "worker_memory" {
  description = "Fargate memory in MiB for the worker task."
  type        = number
  default     = 512
}

variable "app_image_tag" {
  description = "Image tag for the API container."
  type        = string
  default     = "latest"
}

variable "worker_image_tag" {
  description = "Image tag for the worker container."
  type        = string
  default     = "latest"
}

variable "pre_commit_delay_ms" {
  description = "Optional worker-side delay before DynamoDB commit to support crash testing."
  type        = number
  default     = 0
}

variable "locking_mode" {
  description = "Worker-side concurrency control mode."
  type        = string
  default     = "optimistic"
}

variable "lock_ttl_seconds" {
  description = "Lease duration for pessimistic account locks."
  type        = number
  default     = 90
}

variable "metrics_log_interval_seconds" {
  description = "Interval for worker metrics snapshot log lines. 0 disables."
  type        = number
  default     = 30
}
