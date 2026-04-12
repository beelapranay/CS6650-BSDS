variable "aws_region" {
  description = "AWS region for all resources."
  type        = string
}

variable "aws_account_id" {
  description = "AWS account ID used to construct the LabRole ARN."
  type        = string
}

variable "project_name" {
  description = "Short project name used in resource naming."
  type        = string
  default     = "album-store"
}

variable "environment" {
  description = "Environment name used in tags and resource names."
  type        = string
  default     = "dev"
}

variable "lab_role_name" {
  description = "IAM role name to use everywhere. Must remain LabRole."
  type        = string
  default     = "LabRole"

  validation {
    condition     = var.lab_role_name == "LabRole"
    error_message = "Only LabRole is permitted for this infrastructure."
  }
}

variable "provider_assume_lab_role" {
  description = "Whether Terraform should explicitly assume LabRole. Disable this if your lab session already runs as LabRole and self-assume is blocked."
  type        = bool
  default     = true
}

variable "create_lab_instance_profile" {
  description = "Legacy EC2 variable retained only to avoid breaking existing tfvars files. Ignored by the ECS deployment."
  type        = bool
  default     = true
}

variable "existing_lab_instance_profile_name" {
  description = "Legacy EC2 variable retained only to avoid breaking existing tfvars files. Ignored by the ECS deployment."
  type        = string
  default     = null
}

variable "instance_type" {
  description = "Legacy EC2 variable retained only to avoid breaking existing tfvars files. Ignored by the ECS deployment."
  type        = string
  default     = "t3.small"
}

variable "key_name" {
  description = "Legacy EC2 variable retained only to avoid breaking existing tfvars files. Ignored by the ECS deployment."
  type        = string
  default     = null
}

variable "subnet_id" {
  description = "Optional single subnet ID. Prefer subnet_ids for ECS/ALB. If subnet_ids is null and subnet_id is set, it will be used."
  type        = string
  default     = null
}

variable "subnet_ids" {
  description = "Optional subnet IDs for ECS and the ALB. If null, Terraform uses all subnets in the default VPC."
  type        = list(string)
  default     = null
}

variable "allowed_ingress_cidrs" {
  description = "CIDR blocks allowed to reach the HTTP endpoint."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "bucket_name" {
  description = "Optional S3 bucket name. Leave null to generate one."
  type        = string
  default     = null
}

variable "app_image_tag" {
  description = "Container tag to deploy for the API image."
  type        = string
  default     = "latest"
}

variable "worker_image_tag" {
  description = "Container tag to deploy for the worker image."
  type        = string
  default     = "latest"
}

variable "photo_workers" {
  description = "Local thread count for the app when running in local mode. Included here for runtime parity."
  type        = number
  default     = 4
}

variable "s3_presign_ttl_seconds" {
  description = "TTL for presigned S3 URLs returned by the API."
  type        = number
  default     = 3600
}

variable "max_upload_bytes" {
  description = "Maximum photo size supported by the app and worker."
  type        = number
  default     = 209715200
}

variable "log_level" {
  description = "Runtime log level for ECS containers."
  type        = string
  default     = "INFO"
}

variable "api_task_cpu" {
  description = "Fargate CPU units for the API task."
  type        = number
  default     = 512
}

variable "api_task_memory" {
  description = "Fargate memory in MiB for the API task."
  type        = number
  default     = 1024
}

variable "worker_task_cpu" {
  description = "Fargate CPU units for the worker task."
  type        = number
  default     = 512
}

variable "worker_task_memory" {
  description = "Fargate memory in MiB for the worker task."
  type        = number
  default     = 1024
}

variable "api_desired_count" {
  description = "Number of API tasks to run."
  type        = number
  default     = 1
}

variable "worker_desired_count" {
  description = "Number of worker tasks to run."
  type        = number
  default     = 1
}
