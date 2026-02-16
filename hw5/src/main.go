package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type Product struct {
	ProductID   int    `json:"product_id"`
	SKU         string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID  int    `json:"category_id"`
	Weight      int    `json:"weight"`
	SomeOtherID int    `json:"some_other_id"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type productStore struct {
	mu   sync.RWMutex
	data map[int]Product
}

func newProductStore() *productStore {
	return &productStore{data: make(map[int]Product)}
}

func (s *productStore) get(id int) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.data[id]
	return p, ok
}

func (s *productStore) upsert(p Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[p.ProductID] = p
}

func main() {
	store := newProductStore()
	mux := http.NewServeMux()

	mux.HandleFunc("/products/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/products/" || r.URL.Path == "/products" {
			notFound(w)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/products/")
		parts := strings.Split(strings.Trim(path, "/"), "/")

		if len(parts) == 1 && r.Method == http.MethodGet {
			productID, ok := parsePositiveInt(parts[0])
			if !ok {
				notFound(w)
				return
			}
			handleGetProduct(w, r, store, productID)
			return
		}

		if len(parts) == 2 && parts[1] == "details" && r.Method == http.MethodPost {
			productID, ok := parsePositiveInt(parts[0])
			if !ok {
				writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid product ID", "productId must be a positive integer")
				return
			}
			handlePostProductDetails(w, r, store, productID)
			return
		}

		notFound(w)
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("listening on :8080")
	log.Fatal(server.ListenAndServe())
}

func handleGetProduct(w http.ResponseWriter, r *http.Request, store *productStore, productID int) {
	p, ok := store.get(productID)
	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Product not found", "No product with that ID")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func handlePostProductDetails(w http.ResponseWriter, r *http.Request, store *productStore, productID int) {
	var p Product
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", err.Error())
		return
	}

	if p.ProductID != productID {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", "product_id must match path productId")
		return
	}

	if err := validateProduct(p); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", err.Error())
		return
	}

	store.upsert(p)
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	if dec.More() {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

func validateProduct(p Product) error {
	if p.ProductID < 1 {
		return errors.New("product_id must be >= 1")
	}
	if len(strings.TrimSpace(p.SKU)) == 0 {
		return errors.New("sku is required")
	}
	if len(p.SKU) > 100 {
		return errors.New("sku must be <= 100 characters")
	}
	if len(strings.TrimSpace(p.Manufacturer)) == 0 {
		return errors.New("manufacturer is required")
	}
	if len(p.Manufacturer) > 200 {
		return errors.New("manufacturer must be <= 200 characters")
	}
	if p.CategoryID < 1 {
		return errors.New("category_id must be >= 1")
	}
	if p.Weight < 0 {
		return errors.New("weight must be >= 0")
	}
	if p.SomeOtherID < 1 {
		return errors.New("some_other_id must be >= 1")
	}
	return nil
}

func parsePositiveInt(raw string) (int, bool) {
	id, err := strconv.Atoi(raw)
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message, details string) {
	writeJSON(w, status, ErrorResponse{
		Error:   code,
		Message: message,
		Details: details,
	})
}

func notFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "NOT_FOUND", "Not found", "Resource does not exist")
}
