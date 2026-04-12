output "lab_role_arn" {
  description = "The only role ARN used by Terraform."
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

output "photo_bucket_name" {
  description = "S3 bucket used for staged and processed photos."
  value       = aws_s3_bucket.photos.bucket
}

output "photo_queue_url" {
  description = "SQS queue URL used for photo processing jobs."
  value       = aws_sqs_queue.photo_jobs.id
}

output "albums_table_name" {
  value = aws_dynamodb_table.albums.name
}

output "photos_table_name" {
  value = aws_dynamodb_table.photos.name
}

output "photo_counters_table_name" {
  value = aws_dynamodb_table.photo_counters.name
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
  description = "Base URL to submit to ChaosArena after deployment is working."
  value       = "http://${aws_lb.api.dns_name}"
}
