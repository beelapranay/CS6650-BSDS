variable "table_name" {
  description = "DynamoDB table name."
  type        = string
}

variable "tags" {
  description = "Tags applied to the DynamoDB table."
  type        = map(string)
}
