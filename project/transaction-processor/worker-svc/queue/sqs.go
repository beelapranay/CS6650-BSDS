package queue

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type Message struct {
	Body          string
	ReceiptHandle string
}

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

// Receive long-polls for up to 10 messages.
func (s *SQSClient) Receive(ctx context.Context) ([]Message, error) {
	out, err := s.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(s.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     20,
	})
	if err != nil {
		return nil, err
	}
	msgs := make([]Message, 0, len(out.Messages))
	for _, m := range out.Messages {
		msgs = append(msgs, Message{
			Body:          aws.ToString(m.Body),
			ReceiptHandle: aws.ToString(m.ReceiptHandle),
		})
	}
	return msgs, nil
}

// Delete removes a message from the queue after successful processing.
func (s *SQSClient) Delete(ctx context.Context, receiptHandle string) error {
	_, err := s.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(s.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	return err
}

func buildConfig(ctx context.Context) (aws.Config, error) {
	optFns := []func(*config.LoadOptions) error{
		config.WithRegion(getEnvOrDefault("AWS_REGION", "us-east-1")),
	}
	if accessKey, secretKey := os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"); accessKey != "" && secretKey != "" {
		optFns = append(optFns, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey,
			secretKey,
			"",
		)))
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
