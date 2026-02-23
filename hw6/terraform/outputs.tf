output "ecs_cluster_name" {
  description = "Name of the created ECS cluster"
  value       = module.ecs.cluster_name
}

output "ecs_service_name" {
  description = "Name of the running ECS service"
  value       = module.ecs.service_name
}

output "alb_dns_name" {
  description = "DNS name for the Application Load Balancer"
  value       = module.alb.alb_dns_name
}

output "alb_target_group_arn" {
  description = "Target group ARN"
  value       = module.alb.target_group_arn
}
