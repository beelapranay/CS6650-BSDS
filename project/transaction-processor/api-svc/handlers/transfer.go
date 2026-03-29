package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"transaction-processor/api-svc/db"
	"transaction-processor/api-svc/models"
	"transaction-processor/api-svc/queue"
)

type Handler struct {
	db    *db.DynamoClient
	queue *queue.SQSClient
}

func NewHandler(dynamo *db.DynamoClient, q *queue.SQSClient) *Handler {
	return &Handler{db: dynamo, queue: q}
}

// POST /transfer
func (h *Handler) PostTransfer(c *gin.Context) {
	var req models.TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.TransactionID == "" || req.FromAccount == "" || req.ToAccount == "" || req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "transaction_id, from_account, to_account, and amount > 0 are required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Idempotency check
	existing, err := h.db.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check idempotency"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusOK, models.TransferResponse{
			TransactionID: existing.TransactionID,
			Status:        existing.Status,
			Message:       "transaction already exists",
		})
		return
	}

	// Write PENDING record with conditional write (attribute_not_exists)
	tx := models.Transaction{
		TransactionID: req.TransactionID,
		FromAccount:   req.FromAccount,
		ToAccount:     req.ToAccount,
		Amount:        req.Amount,
		Status:        "PENDING",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if err := h.db.PutTransactionIfNotExists(ctx, tx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record transaction"})
		return
	}

	// Enqueue
	if err := h.queue.SendMessage(ctx, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enqueue transfer"})
		return
	}

	c.JSON(http.StatusAccepted, models.TransferResponse{
		TransactionID: req.TransactionID,
		Status:        "PENDING",
		Message:       "transfer accepted",
	})
}

// GET /transfer/:id
func (h *Handler) GetTransfer(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.db.GetTransaction(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch transaction"})
		return
	}
	if tx == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}
	c.JSON(http.StatusOK, tx)
}

// GET /health
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
