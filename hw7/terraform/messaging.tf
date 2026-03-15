resource "aws_sns_topic" "order_processing" {
  name = "order-processing-events"

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-sns"
  })
}

resource "aws_sqs_queue" "order_processing" {
  name                       = "order-processing-queue"
  visibility_timeout_seconds = 30
  message_retention_seconds  = 345600
  receive_wait_time_seconds  = 20

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-sqs"
  })
}

data "aws_iam_policy_document" "sqs_allow_sns" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["sns.amazonaws.com"]
    }

    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.order_processing.arn]

    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = [aws_sns_topic.order_processing.arn]
    }
  }
}

resource "aws_sqs_queue_policy" "order_processing" {
  queue_url = aws_sqs_queue.order_processing.id
  policy    = data.aws_iam_policy_document.sqs_allow_sns.json
}

resource "aws_sns_topic_subscription" "order_queue" {
  topic_arn = aws_sns_topic.order_processing.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.order_processing.arn
}
