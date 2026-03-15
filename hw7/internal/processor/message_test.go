package processor

import (
	"encoding/json"
	"testing"

	"hw7/internal/orders"
)

func TestDecodeOrderMessageDirectJSON(t *testing.T) {
	input, err := json.Marshal(orders.Order{OrderID: "ord-1", CustomerID: 123})
	if err != nil {
		t.Fatalf("marshal direct order: %v", err)
	}

	order, err := DecodeOrderMessage(string(input))
	if err != nil {
		t.Fatalf("decode direct order: %v", err)
	}
	if order.OrderID != "ord-1" {
		t.Fatalf("expected order_id ord-1, got %s", order.OrderID)
	}
}

func TestDecodeOrderMessageSNSEnvelope(t *testing.T) {
	payload, err := json.Marshal(orders.Order{OrderID: "ord-2", CustomerID: 456})
	if err != nil {
		t.Fatalf("marshal order: %v", err)
	}

	input, err := json.Marshal(map[string]string{"Message": string(payload)})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	order, err := DecodeOrderMessage(string(input))
	if err != nil {
		t.Fatalf("decode envelope order: %v", err)
	}
	if order.OrderID != "ord-2" {
		t.Fatalf("expected order_id ord-2, got %s", order.OrderID)
	}
}
