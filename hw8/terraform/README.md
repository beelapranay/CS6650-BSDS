# HW8 Terraform

This Terraform layout is a from-scratch baseline for Homework 8 Step I.

It provisions:

- a dedicated VPC with 2 public subnets
- 2 private application subnets for ECS
- 2 private database subnets for RDS
- an Internet Gateway and one NAT Gateway
- an Application Load Balancer
- ECS Fargate cluster, task definition, and service
- an ECR repository and CloudWatch log group
- an RDS MySQL 8.0 instance on `db.t3.micro`

The ECS service is wired to a tiny Go health-check container in `../src`, which gives the
infrastructure a runnable baseline before the shopping cart endpoints are added in Step I Part 3.

RDS is locked down to the ECS task security group only. The instance is configured with:

- `skip_final_snapshot = true`
- `deletion_protection = false`

Suggested workflow:

1. create `terraform.tfvars` from `terraform.tfvars.example`
2. run `terraform init`
3. run `terraform apply`
4. connect to the RDS endpoint and run `../db/mysql/schema.sql`
