//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	numAccounts    = 100
	initialBalance = 10000.00
	hotAccountID   = "hot-account-001"
)

func main() {
	ctx := context.Background()

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

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	client := dynamodb.NewFromConfig(cfg)
	table := os.Getenv("DYNAMODB_ACCOUNTS_TABLE")
	if table == "" {
		table = "accounts"
	}

	accountIDs := make([]string, 0, numAccounts+1)

	// Seed numbered accounts
	for i := 0; i < numAccounts; i++ {
		id := fmt.Sprintf("account-%d", i)
		if err := putAccount(ctx, client, table, id, initialBalance); err != nil {
			log.Fatalf("failed to seed %s: %v", id, err)
		}
		accountIDs = append(accountIDs, id)
	}

	// Seed hot account (used for Experiment 1)
	if err := putAccount(ctx, client, table, hotAccountID, initialBalance); err != nil {
		log.Fatalf("failed to seed hot account: %v", err)
	}
	accountIDs = append(accountIDs, hotAccountID)

	// Write account IDs to accounts.json for Locust
	f, err := os.Create("load-tests/accounts.json")
	if err != nil {
		log.Fatalf("failed to create accounts.json: %v", err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(accountIDs); err != nil {
		log.Fatalf("failed to write accounts.json: %v", err)
	}

	total := float64(numAccounts+1) * initialBalance
	fmt.Printf("Seeded %d accounts (+ 1 hot account) with balance=%.2f each\n", numAccounts, initialBalance)
	fmt.Printf("Expected total balance: %.2f\n", total)
	fmt.Println("Account IDs written to load-tests/accounts.json")
}

func putAccount(ctx context.Context, client *dynamodb.Client, table, id string, balance float64) error {
	_, err := client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item: map[string]types.AttributeValue{
			"account_id": &types.AttributeValueMemberS{Value: id},
			"balance":    &types.AttributeValueMemberN{Value: strconv.FormatFloat(balance, 'f', -1, 64)},
			"version":    &types.AttributeValueMemberN{Value: "0"},
		},
	})
	return err
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
