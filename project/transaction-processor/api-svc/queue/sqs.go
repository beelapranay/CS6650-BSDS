package queue

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"transaction-processor/api-svc/models"
)

type SQSClient struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSClient(ctx context.Context) (*SQSClient, error) {
	cfg, err := buildConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &SQSClient{
		client:   sqs.NewFromConfig(cfg),
		queueURL: os.Getenv("SQS_QUEUE_URL"),
	}, nil
}

func (s *SQSClient) SendMessage(ctx context.Context, payload models.TransferRequest) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.queueURL),
		MessageBody: aws.String(string(body)),
	})
	return err
}

func buildConfig(ctx context.Context) (aws.Config, error) {
	optFns := []func(*config.LoadOptions) error{
		config.WithRegion(getEnvOrDefault("AWS_REGION", "us-east-1")),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			getEnvOrDefault("AWS_ACCESS_KEY_ID", "test"),
			getEnvOrDefault("AWS_SECRET_ACCESS_KEY", "test"),
			"",
		)),
	}
	if endpoint := os.Getenv("SQS_ENDPOINT_URL"); endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpoint, HostnameImmutable: true}, nil
			},
		)
		optFns = append(optFns, config.WithEndpointResolverWithOptions(customResolver))
	}
	return config.LoadDefaultConfig(ctx, optFns...)
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
