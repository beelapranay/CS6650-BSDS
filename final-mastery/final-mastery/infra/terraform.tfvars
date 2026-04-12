aws_region   = "us-west-2"
aws_account_id = "770561854863"

lab_role_name = "LabRole"
provider_assume_lab_role = false

# Legacy EC2 variables. Ignored by the ECS deployment path.
create_lab_instance_profile       = false
existing_lab_instance_profile_name = "LabInstanceProfile"

project_name = "album-store"
environment  = "dev"

# Leave subnet_ids unset to use all subnets in the default VPC.
# If Terraform later complains that fewer than 2 subnets are available,
# add two public subnet IDs here.
# subnet_ids = ["subnet-aaaaaaa", "subnet-bbbbbbb"]

app_image_tag    = "latest"
worker_image_tag = "latest"

api_task_cpu        = 4096
api_task_memory     = 8192
worker_task_cpu     = 256
worker_task_memory  = 512

api_desired_count    = 3
worker_desired_count = 0
