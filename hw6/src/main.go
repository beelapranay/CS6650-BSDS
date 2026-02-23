package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type productStore struct {
	data sync.Map
}

func newProductStore() *productStore {
	return &productStore{}
}

func (s *productStore) put(p Product) {
	s.data.Store(p.ID, p)
}

type searchResponse struct {
	Products   []Product `json:"products"`
	TotalFound int       `json:"total_found"`
	SearchTime string    `json:"search_time,omitempty"`
}

func main() {
	store := newProductStore()
	products := generateProducts()
	for _, p := range products {
		store.put(p)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/products/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "use GET")
			return
		}
		handleSearch(w, r, products)
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("listening on :8080")
	log.Fatal(server.ListenAndServe())
}

func handleSearch(w http.ResponseWriter, r *http.Request, products []Product) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Missing query", "q is required")
		return
	}

	start := time.Now()
	query = strings.ToLower(query)

	results := make([]Product, 0, 20)
	totalFound := 0

	limit := 10000
	if len(products) < limit {
		limit = len(products)
	}

	for i := 0; i < limit; i++ {
		p := products[i]
		nameMatch := strings.Contains(strings.ToLower(p.Name), query)
		categoryMatch := strings.Contains(strings.ToLower(p.Category), query)
		if nameMatch || categoryMatch {
			totalFound++
			if len(results) < 20 {
				results = append(results, p)
			}
		}
	}

	resp := searchResponse{
		Products:   results,
		TotalFound: totalFound,
		SearchTime: time.Since(start).String(),
	}
	writeJSON(w, http.StatusOK, resp)
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

func generateProducts() []Product {
	brands := []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"}
	categories := []string{"Electronics", "Books", "Home", "Toys", "Sports", "Grocery", "Clothing"}
	products := make([]Product, 100000)

	for i := 0; i < 100000; i++ {
		brand := brands[i%len(brands)]
		category := categories[i%len(categories)]
		id := i + 1
		products[i] = Product{
			ID:          id,
			Name:        fmt.Sprintf("Product %s %d", brand, id),
			Category:    category,
			Description: fmt.Sprintf("Description for Product %s %d", brand, id),
			Brand:       brand,
		}
	}
	return products
}
