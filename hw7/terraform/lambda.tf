resource "aws_cloudwatch_log_group" "lambda" {
  count = var.enable_lambda ? 1 : 0

  name              = "/aws/lambda/${local.name_prefix}-order-lambda"
  retention_in_days = 14
  tags              = local.common_tags
}

resource "aws_lambda_function" "order_processor" {
  count = var.enable_lambda ? 1 : 0

  function_name    = "${local.name_prefix}-order-lambda"
  role             = local.lab_role_arn
  filename         = var.lambda_zip_path
  source_code_hash = filebase64sha256(var.lambda_zip_path)
  handler          = "bootstrap"
  runtime          = "provided.al2"
  architectures    = ["x86_64"]
  memory_size      = 512
  timeout          = 30

  environment {
    variables = {
      PAYMENT_DELAY = "3s"
    }
  }

  tags = local.common_tags

  depends_on = [aws_cloudwatch_log_group.lambda]
}

resource "aws_lambda_permission" "allow_sns" {
  count = var.enable_lambda ? 1 : 0

  statement_id  = "AllowExecutionFromSNS"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.order_processor[0].function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.order_processing.arn
}

resource "aws_sns_topic_subscription" "lambda" {
  count = var.enable_lambda ? 1 : 0

  topic_arn = aws_sns_topic.order_processing.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.order_processor[0].arn

  depends_on = [aws_lambda_permission.allow_sns]
}
