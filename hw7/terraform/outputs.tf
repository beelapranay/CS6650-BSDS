output "alb_dns_name" {
  description = "Public DNS name for the order receiver"
  value       = aws_lb.receiver.dns_name
}

output "order_events_topic_arn" {
  description = "SNS topic ARN for async order publishing"
  value       = aws_sns_topic.order_processing.arn
}

output "order_queue_url" {
  description = "SQS queue URL consumed by the processor"
  value       = aws_sqs_queue.order_processing.id
}

output "lambda_function_name" {
  description = "Part III Lambda function name"
  value       = var.enable_lambda ? aws_lambda_function.order_processor[0].function_name : null
}
