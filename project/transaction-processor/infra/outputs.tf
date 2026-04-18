output "aws_region" {
  description = "AWS region used by the stack."
  value       = var.aws_region
}

output "lab_role_arn" {
  description = "LabRole ARN used by Terraform and ECS."
  value       = local.lab_role_arn
}

output "api_ecr_repository_url" {
  description = "ECR repository URL for the API image."
  value       = aws_ecr_repository.api.repository_url
}

output "worker_ecr_repository_url" {
  description = "ECR repository URL for the worker image."
  value       = aws_ecr_repository.worker.repository_url
}

output "accounts_table_name" {
  description = "DynamoDB accounts table name."
  value       = aws_dynamodb_table.accounts.name
}

output "transactions_table_name" {
  description = "DynamoDB transactions table name."
  value       = aws_dynamodb_table.transactions.name
}

output "account_locks_table_name" {
  description = "DynamoDB lock table name used by pessimistic mode."
  value       = aws_dynamodb_table.account_locks.name
}

output "queue_url" {
  description = "Primary SQS queue URL for transfers."
  value       = aws_sqs_queue.transfers.id
}

output "dlq_url" {
  description = "Dead-letter queue URL for failed transfer retries."
  value       = aws_sqs_queue.transfers_dlq.id
}

output "ecs_cluster_name" {
  description = "ECS cluster name."
  value       = aws_ecs_cluster.main.name
}

output "api_service_name" {
  description = "ECS service name for the API."
  value       = aws_ecs_service.api.name
}

output "worker_service_name" {
  description = "ECS service name for the worker."
  value       = aws_ecs_service.worker.name
}

output "alb_dns_name" {
  description = "Public DNS name of the ALB."
  value       = aws_lb.api.dns_name
}

output "base_url" {
  description = "Base URL for the deployed API."
  value       = "http://${aws_lb.api.dns_name}"
}
