package messaging

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"hw7/internal/orders"
)

type Publisher interface {
	PublishOrder(ctx context.Context, order orders.Order) error
}

type LoggingPublisher struct {
	logger *slog.Logger
}

func (p *LoggingPublisher) PublishOrder(ctx context.Context, order orders.Order) error {
	p.logger.InfoContext(ctx, "order accepted without SNS topic configured", "order_id", order.OrderID)
	return nil
}

type SNSPublisher struct {
	client   *sns.Client
	logger   *slog.Logger
	topicARN string
}

func NewPublisher(ctx context.Context, topicARN string, logger *slog.Logger) (Publisher, error) {
	if topicARN == "" {
		return &LoggingPublisher{logger: logger}, nil
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &SNSPublisher{
		client:   sns.NewFromConfig(cfg),
		logger:   logger,
		topicARN: topicARN,
	}, nil
}

func (p *SNSPublisher) PublishOrder(ctx context.Context, order orders.Order) error {
	payload, err := json.Marshal(order)
	if err != nil {
		return err
	}

	_, err = p.client.Publish(ctx, &sns.PublishInput{
		Message:  aws.String(string(payload)),
		TopicArn: &p.topicARN,
	})
	if err != nil {
		return err
	}

	p.logger.InfoContext(ctx, "published order to SNS", "order_id", order.OrderID, "topic_arn", p.topicARN)
	return nil
}
