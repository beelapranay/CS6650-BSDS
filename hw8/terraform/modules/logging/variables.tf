variable "log_group_name" {
  description = "CloudWatch log group name."
  type        = string
}

variable "retention_in_days" {
  description = "Retention in days."
  type        = number
}

variable "tags" {
  description = "Tags applied to the log group."
  type        = map(string)
}
