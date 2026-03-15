package orders

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

type Item struct {
	SKU        string `json:"sku"`
	Quantity   int    `json:"quantity"`
	PriceCents int    `json:"price_cents"`
}

type Order struct {
	OrderID    string    `json:"order_id"`
	CustomerID int       `json:"customer_id"`
	Status     string    `json:"status"`
	Items      []Item    `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateOrderRequest struct {
	CustomerID int    `json:"customer_id"`
	Items      []Item `json:"items"`
}

func (r CreateOrderRequest) Validate() error {
	if r.CustomerID <= 0 {
		return errors.New("customer_id must be greater than zero")
	}
	if len(r.Items) == 0 {
		return errors.New("items must contain at least one entry")
	}
	for _, item := range r.Items {
		if item.SKU == "" {
			return errors.New("item sku is required")
		}
		if item.Quantity <= 0 {
			return errors.New("item quantity must be greater than zero")
		}
		if item.PriceCents < 0 {
			return errors.New("item price_cents cannot be negative")
		}
	}
	return nil
}

func NewOrder(req CreateOrderRequest) Order {
	return Order{
		OrderID:    newOrderID(),
		CustomerID: req.CustomerID,
		Status:     StatusPending,
		Items:      req.Items,
		CreatedAt:  time.Now().UTC(),
	}
}

func newOrderID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "ord-fallback"
	}
	return "ord-" + hex.EncodeToString(buf)
}
