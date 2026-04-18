package db

import (
	"context"
	"errors"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"transaction-processor/api-svc/models"
)

type DynamoClient struct {
	client  *dynamodb.Client
	txTable string
}

func NewDynamoClient(ctx context.Context) (*DynamoClient, error) {
	cfg, err := buildConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &DynamoClient{
		client:  dynamodb.NewFromConfig(cfg),
		txTable: os.Getenv("DYNAMODB_TRANSACTIONS_TABLE"),
	}, nil
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
	if endpoint := os.Getenv("DYNAMODB_ENDPOINT_URL"); endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpoint, HostnameImmutable: true}, nil
			},
		)
		optFns = append(optFns, config.WithEndpointResolverWithOptions(customResolver))
	}
	return config.LoadDefaultConfig(ctx, optFns...)
}

func (c *DynamoClient) GetTransaction(ctx context.Context, id string) (*models.Transaction, error) {
	out, err := c.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.txTable),
		Key: map[string]types.AttributeValue{
			"transaction_id": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	var tx models.Transaction
	if err := attributevalue.UnmarshalMap(out.Item, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (c *DynamoClient) PutTransactionIfNotExists(ctx context.Context, tx models.Transaction) (bool, error) {
	av, err := attributevalue.MarshalMap(tx)
	if err != nil {
		return false, err
	}
	_, err = c.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(c.txTable),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(transaction_id)"),
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
