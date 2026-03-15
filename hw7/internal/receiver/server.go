package receiver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"hw7/internal/messaging"
	"hw7/internal/orders"
)

type Server struct {
	logger    *slog.Logger
	publisher messaging.Publisher
	payments  *orders.PaymentSimulator
}

type orderResponse struct {
	Order   orders.Order `json:"order"`
	Message string       `json:"message,omitempty"`
}

func NewServer(logger *slog.Logger, publisher messaging.Publisher, payments *orders.PaymentSimulator) *Server {
	return &Server{
		logger:    logger,
		publisher: publisher,
		payments:  payments,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/orders/sync", s.handleSyncOrder)
	mux.HandleFunc("/orders/async", s.handleAsyncOrder)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) handleSyncOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	order, err := decodeOrderRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	order.Status = orders.StatusProcessing
	if err := s.payments.Process(r.Context(), order); err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			statusCode = http.StatusRequestTimeout
		}
		writeError(w, statusCode, err.Error())
		return
	}

	order.Status = orders.StatusCompleted
	writeJSON(w, http.StatusOK, orderResponse{
		Order:   order,
		Message: "order processed synchronously",
	})
}

func (s *Server) handleAsyncOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	order, err := decodeOrderRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	order.Status = orders.StatusPending
	if err := s.publisher.PublishOrder(r.Context(), order); err != nil {
		s.logger.ErrorContext(r.Context(), "failed to publish order", "order_id", order.OrderID, "error", err)
		writeError(w, http.StatusBadGateway, "failed to publish order event")
		return
	}

	writeJSON(w, http.StatusAccepted, orderResponse{
		Order:   order,
		Message: "order accepted for asynchronous processing",
	})
}

func decodeOrderRequest(r *http.Request) (orders.Order, error) {
	defer r.Body.Close()

	var req orders.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return orders.Order{}, err
	}
	if err := req.Validate(); err != nil {
		return orders.Order{}, err
	}
	return orders.NewOrder(req), nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error":     message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
