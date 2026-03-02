package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type SearchResp struct {
	Query     string          `json:"query"`
	SKU       string          `json:"sku"`
	Inventory json.RawMessage `json:"inventory,omitempty"`
	Note      string          `json:"note"`
}

func main() {
	inventoryURL := getenv("INVENTORY_URL", "http://localhost:8081")

	client := &http.Client{}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, "missing q", 400)
			return
		}
		sku := "SKU-123"

		// 🔥 FAIL FAST: 200ms timeout
		ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
		defer cancel()

		req, _ := http.NewRequestWithContext(ctx, "GET", inventoryURL+"/inventory?sku="+sku, nil)
		resp, err := client.Do(req)

		if err != nil {
			// Timeout or downstream error
			out := SearchResp{
				Query: q,
				SKU:   sku,
				Note:  "FIXED: timeout fallback (inventory omitted)",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(out)
			return
		}

		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 500 {
			out := SearchResp{
				Query: q,
				SKU:   sku,
				Note:  "FIXED: downstream 5xx fallback",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(out)
			return
		}

		out := SearchResp{
			Query:     q,
			SKU:       sku,
			Inventory: body,
			Note:      "FIXED: fast timeout applied",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})

	log.Println("search-service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}