package models

type TransferRequest struct {
	TransactionID string  `json:"transaction_id"`
	FromAccount   string  `json:"from_account"`
	ToAccount     string  `json:"to_account"`
	Amount        float64 `json:"amount"`
}

type TransferResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

type Transaction struct {
	TransactionID string  `json:"transaction_id" dynamodbav:"transaction_id"`
	FromAccount   string  `json:"from_account"   dynamodbav:"from_account"`
	ToAccount     string  `json:"to_account"     dynamodbav:"to_account"`
	Amount        float64 `json:"amount"         dynamodbav:"amount"`
	Status        string  `json:"status"         dynamodbav:"status"`
	CreatedAt     string  `json:"created_at"     dynamodbav:"created_at"`
}
