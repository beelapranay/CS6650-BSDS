package processor

import (
	"encoding/json"
	"errors"

	"hw7/internal/orders"
)

type snsEnvelope struct {
	Message string `json:"Message"`
}

func DecodeOrderMessage(body string) (orders.Order, error) {
	var order orders.Order
	if err := json.Unmarshal([]byte(body), &order); err == nil && order.OrderID != "" {
		return order, nil
	}

	var envelope snsEnvelope
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		return orders.Order{}, err
	}
	if envelope.Message == "" {
		return orders.Order{}, errors.New("SQS message missing SNS envelope payload")
	}
	if err := json.Unmarshal([]byte(envelope.Message), &order); err != nil {
		return orders.Order{}, err
	}
	if order.OrderID == "" {
		return orders.Order{}, errors.New("order_id is required in message")
	}
	return order, nil
}
