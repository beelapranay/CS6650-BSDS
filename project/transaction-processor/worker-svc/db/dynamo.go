package db

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"transaction-processor/api-svc/models"
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
	locksTable    string
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
		locksTable:    os.Getenv("DYNAMODB_LOCKS_TABLE"),
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

func (c *DynamoClient) GetTransaction(ctx context.Context, id string) (*models.Transaction, error) {
	out, err := c.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.txTable),
		Key: map[string]types.AttributeValue{
			"transaction_id": &types.AttributeValueMemberS{Value: id},
		},
		ConsistentRead: aws.Bool(true),
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

func (c *DynamoClient) AcquireAccountLock(ctx context.Context, accountID, ownerTxID string, now time.Time, ttl time.Duration) error {
	expiresAt := now.Add(ttl).Unix()
	_, err := c.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.locksTable),
		Item: map[string]types.AttributeValue{
			"account_id":  &types.AttributeValueMemberS{Value: accountID},
			"owner_tx_id": &types.AttributeValueMemberS{Value: ownerTxID},
			"expires_at":  &types.AttributeValueMemberN{Value: strconv.FormatInt(expiresAt, 10)},
		},
		ConditionExpression: aws.String("attribute_not_exists(account_id) OR expires_at < :now OR owner_tx_id = :owner"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now":   &types.AttributeValueMemberN{Value: strconv.FormatInt(now.Unix(), 10)},
			":owner": &types.AttributeValueMemberS{Value: ownerTxID},
		},
	})
	return err
}

func (c *DynamoClient) ReleaseAccountLock(ctx context.Context, accountID, ownerTxID string) error {
	_, err := c.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(c.locksTable),
		Key: map[string]types.AttributeValue{
			"account_id": &types.AttributeValueMemberS{Value: accountID},
		},
		ConditionExpression: aws.String("owner_tx_id = :owner"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":owner": &types.AttributeValueMemberS{Value: ownerTxID},
		},
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return nil
		}
	}
	return err
}

func (c *DynamoClient) MarkTransactionFailedIfPending(ctx context.Context, id string) (bool, error) {
	_, err := c.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.txTable),
		Key: map[string]types.AttributeValue{
			"transaction_id": &types.AttributeValueMemberS{Value: id},
		},
		UpdateExpression:    aws.String("SET #s = :failed"),
		ConditionExpression: aws.String("#s = :pending"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pending": &types.AttributeValueMemberS{Value: "PENDING"},
			":failed":  &types.AttributeValueMemberS{Value: "FAILED"},
		},
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

func (c *DynamoClient) CompleteTransfer(ctx context.Context, txID string, sender *Account, receiver *Account, amount float64) error {
	_, err := c.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Update: &types.Update{
					TableName: aws.String(c.accountsTable),
					Key: map[string]types.AttributeValue{
						"account_id": &types.AttributeValueMemberS{Value: sender.AccountID},
					},
					UpdateExpression:    aws.String("SET balance = :new_balance, #ver = :new_version"),
					ConditionExpression: aws.String("#ver = :expected_version AND balance >= :amount"),
					ExpressionAttributeNames: map[string]string{
						"#ver": "version",
					},
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":new_balance":      &types.AttributeValueMemberN{Value: formatFloat(sender.Balance - amount)},
						":new_version":      &types.AttributeValueMemberN{Value: formatInt(sender.Version + 1)},
						":expected_version": &types.AttributeValueMemberN{Value: formatInt(sender.Version)},
						":amount":           &types.AttributeValueMemberN{Value: formatFloat(amount)},
					},
				},
			},
			{
				Update: &types.Update{
					TableName: aws.String(c.accountsTable),
					Key: map[string]types.AttributeValue{
						"account_id": &types.AttributeValueMemberS{Value: receiver.AccountID},
					},
					UpdateExpression:    aws.String("SET balance = :new_balance, #ver = :new_version"),
					ConditionExpression: aws.String("#ver = :expected_version"),
					ExpressionAttributeNames: map[string]string{
						"#ver": "version",
					},
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":new_balance":      &types.AttributeValueMemberN{Value: formatFloat(receiver.Balance + amount)},
						":new_version":      &types.AttributeValueMemberN{Value: formatInt(receiver.Version + 1)},
						":expected_version": &types.AttributeValueMemberN{Value: formatInt(receiver.Version)},
					},
				},
			},
			{
				Update: &types.Update{
					TableName: aws.String(c.txTable),
					Key: map[string]types.AttributeValue{
						"transaction_id": &types.AttributeValueMemberS{Value: txID},
					},
					UpdateExpression:    aws.String("SET #s = :completed"),
					ConditionExpression: aws.String("#s = :pending"),
					ExpressionAttributeNames: map[string]string{
						"#s": "status",
					},
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":pending":   &types.AttributeValueMemberS{Value: "PENDING"},
						":completed": &types.AttributeValueMemberS{Value: "COMPLETED"},
					},
				},
			},
		},
	})
	return err
}

func (c *DynamoClient) CompleteTransferPessimistic(ctx context.Context, txID string, sender *Account, receiver *Account, amount float64) error {
	items := []types.TransactWriteItem{
		{
			Update: &types.Update{
				TableName: aws.String(c.accountsTable),
				Key: map[string]types.AttributeValue{
					"account_id": &types.AttributeValueMemberS{Value: sender.AccountID},
				},
				UpdateExpression:    aws.String("SET balance = :new_balance, #ver = :new_version"),
				ConditionExpression: aws.String("balance >= :amount"),
				ExpressionAttributeNames: map[string]string{
					"#ver": "version",
				},
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":new_balance": &types.AttributeValueMemberN{Value: formatFloat(sender.Balance - amount)},
					":new_version": &types.AttributeValueMemberN{Value: formatInt(sender.Version + 1)},
					":amount":      &types.AttributeValueMemberN{Value: formatFloat(amount)},
				},
			},
		},
		{
			Update: &types.Update{
				TableName: aws.String(c.accountsTable),
				Key: map[string]types.AttributeValue{
					"account_id": &types.AttributeValueMemberS{Value: receiver.AccountID},
				},
				UpdateExpression: aws.String("SET balance = :new_balance, #ver = :new_version"),
				ExpressionAttributeNames: map[string]string{
					"#ver": "version",
				},
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":new_balance": &types.AttributeValueMemberN{Value: formatFloat(receiver.Balance + amount)},
					":new_version": &types.AttributeValueMemberN{Value: formatInt(receiver.Version + 1)},
				},
			},
		},
		{
			Update: &types.Update{
				TableName: aws.String(c.txTable),
				Key: map[string]types.AttributeValue{
					"transaction_id": &types.AttributeValueMemberS{Value: txID},
				},
				UpdateExpression:    aws.String("SET #s = :completed"),
				ConditionExpression: aws.String("#s = :pending"),
				ExpressionAttributeNames: map[string]string{
					"#s": "status",
				},
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":pending":   &types.AttributeValueMemberS{Value: "PENDING"},
					":completed": &types.AttributeValueMemberS{Value: "COMPLETED"},
				},
			},
		},
		{
			Delete: &types.Delete{
				TableName: aws.String(c.locksTable),
				Key: map[string]types.AttributeValue{
					"account_id": &types.AttributeValueMemberS{Value: sender.AccountID},
				},
				ConditionExpression: aws.String("owner_tx_id = :owner"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":owner": &types.AttributeValueMemberS{Value: txID},
				},
			},
		},
		{
			Delete: &types.Delete{
				TableName: aws.String(c.locksTable),
				Key: map[string]types.AttributeValue{
					"account_id": &types.AttributeValueMemberS{Value: receiver.AccountID},
				},
				ConditionExpression: aws.String("owner_tx_id = :owner"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":owner": &types.AttributeValueMemberS{Value: txID},
				},
			},
		},
	}

	_, err := c.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: items,
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
