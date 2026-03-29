output "endpoint" {
  description = "MySQL endpoint."
  value       = aws_db_instance.this.address
}

output "port" {
  description = "MySQL port."
  value       = aws_db_instance.this.port
}

output "db_name" {
  description = "Initial database name."
  value       = aws_db_instance.this.db_name
}

output "security_group_id" {
  description = "Security group protecting the MySQL instance."
  value       = aws_security_group.this.id
}

output "subnet_group_name" {
  description = "DB subnet group name."
  value       = aws_db_subnet_group.this.name
}
