output "alb_dns_name" {
  description = "Public DNS name of the application load balancer."
  value       = module.alb.alb_dns_name
}

output "ecs_cluster_name" {
  description = "Name of the ECS cluster."
  value       = module.ecs.cluster_name
}

output "ecs_service_name" {
  description = "Name of the ECS service."
  value       = module.ecs.service_name
}

output "ecr_repository_url" {
  description = "ECR repository URL for the shopping cart API image."
  value       = module.ecr.repository_url
}

output "db_endpoint" {
  description = "DNS endpoint for the MySQL instance."
  value       = module.rds_mysql.endpoint
}

output "db_port" {
  description = "Port exposed by the MySQL instance."
  value       = module.rds_mysql.port
}

output "db_name" {
  description = "Database name provisioned for the shopping cart service."
  value       = module.rds_mysql.db_name
}

output "vpc_id" {
  description = "ID of the Homework 8 VPC."
  value       = module.network.vpc_id
}

output "dynamodb_table_name" {
  description = "DynamoDB table name for the shopping cart service."
  value       = module.dynamodb.table_name
}

output "dynamodb_table_arn" {
  description = "ARN of the DynamoDB table for the shopping cart service."
  value       = module.dynamodb.table_arn
}
