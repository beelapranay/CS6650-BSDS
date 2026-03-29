variable "repository_name" {
  description = "ECR repository name."
  type        = string
}

variable "tags" {
  description = "Tags applied to the repository."
  type        = map(string)
}
