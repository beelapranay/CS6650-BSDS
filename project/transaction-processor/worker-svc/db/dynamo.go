package db

import (
	"context"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Account struct {
	AccountID string  `dynamodbav:"account_id"`
	Balance   float64 `dynamodbav:"balance"`
	Version   int     `dynamodbav:"version"`
}

type DynamoClient struct {
	client        *dynamodb.Client
	accountsTable string
	txTable       string
}

func NewDynamoClient(ctx context.Context) (*DynamoClient, error) {
	cfg, err := buildConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &DynamoClient{
		client:        dynamodb.NewFromConfig(cfg),
		accountsTable: os.Getenv("DYNAMODB_ACCOUNTS_TABLE"),
		txTable:       os.Getenv("DYNAMODB_TRANSACTIONS_TABLE"),
	}, nil
}

func (c *DynamoClient) GetAccount(ctx context.Context, id string) (*Account, error) {
	out, err := c.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.accountsTable),
		Key: map[string]types.AttributeValue{
			"account_id": &types.AttributeValueMemberS{Value: id},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	var acct Account
	if err := attributevalue.UnmarshalMap(out.Item, &acct); err != nil {
		return nil, err
	}
	return &acct, nil
}

// UpdateBalanceOptimistic does a conditional update: only succeeds if version matches.
// Returns a ConditionalCheckFailedException (wrapped) if version has changed.
func (c *DynamoClient) UpdateBalanceOptimistic(ctx context.Context, id string, newBalance float64, expectedVersion int) error {
	_, err := c.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.accountsTable),
		Key: map[string]types.AttributeValue{
			"account_id": &types.AttributeValueMemberS{Value: id},
		},
		UpdateExpression:    aws.String("SET balance = :new_balance, #ver = :new_version"),
		ConditionExpression: aws.String("#ver = :expected_version"),
		ExpressionAttributeNames: map[string]string{
			"#ver": "version",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":new_balance":       &types.AttributeValueMemberN{Value: formatFloat(newBalance)},
			":new_version":       &types.AttributeValueMemberN{Value: formatInt(expectedVersion + 1)},
			":expected_version":  &types.AttributeValueMemberN{Value: formatInt(expectedVersion)},
		},
	})
	return err
}

func (c *DynamoClient) UpdateTransactionStatus(ctx context.Context, id string, status string) error {
	_, err := c.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.txTable),
		Key: map[string]types.AttributeValue{
			"transaction_id": &types.AttributeValueMemberS{Value: id},
		},
		UpdateExpression: aws.String("SET #s = :status"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: status},
		},
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

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func formatInt(i int) string {
	return strconv.Itoa(i)
}
