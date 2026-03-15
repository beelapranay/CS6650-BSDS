variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Project prefix"
  type        = string
  default     = "hw7"
}

variable "environment" {
  description = "Deployment environment name"
  type        = string
  default     = "dev"
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "public_subnet_cidrs" {
  description = "Public subnet CIDR blocks for the ALB"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "private_subnet_cidrs" {
  description = "Private subnet CIDR blocks for ECS"
  type        = list(string)
  default     = ["10.0.10.0/24", "10.0.11.0/24"]
}

variable "app_port" {
  description = "Port exposed by the receiver container"
  type        = number
  default     = 8080
}

variable "receiver_image_uri" {
  description = "Full ECR image URI for the order receiver"
  type        = string
}

variable "processor_image_uri" {
  description = "Full ECR image URI for the order processor"
  type        = string
}

variable "lambda_zip_path" {
  description = "Path to the built Lambda zip archive"
  type        = string
  default     = "../dist/order-lambda.zip"
}

variable "receiver_desired_count" {
  description = "Desired task count for the receiver service"
  type        = number
  default     = 1
}

variable "processor_desired_count" {
  description = "Desired task count for the processor service"
  type        = number
  default     = 1
}

variable "sync_payment_workers" {
  description = "Concurrent payment slots for the synchronous API path"
  type        = number
  default     = 1
}

variable "processor_workers" {
  description = "Concurrent workers inside the order processor task"
  type        = number
  default     = 1
}

variable "enable_lambda" {
  description = "Whether to deploy the Part III Lambda subscriber"
  type        = bool
  default     = false
}

variable "lab_role_name" {
  description = "Existing IAM role provided by the lab environment"
  type        = string
  default     = "LabRole"
}
