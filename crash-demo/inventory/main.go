package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Resp struct {
	SKU       string `json:"sku"`
	InStock   bool   `json:"inStock"`
	LatencyMs int    `json:"latencyMs"`
}

func main() {
	rand.Seed(time.Now().UnixNano())

	minSleepMs := getenvInt("MIN_SLEEP_MS", 50)
	maxSleepMs := getenvInt("MAX_SLEEP_MS", 2500)
	failPct := getenvInt("FAIL_PCT", 20)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/inventory", func(w http.ResponseWriter, r *http.Request) {
		sku := r.URL.Query().Get("sku")
		if sku == "" {
			http.Error(w, "missing sku", 400)
			return
		}

		sleep := rand.Intn(maxSleepMs-minSleepMs+1) + minSleepMs
		time.Sleep(time.Duration(sleep) * time.Millisecond)

		if rand.Intn(100) < failPct {
			http.Error(w, "inventory service failed", 500)
			return
		}

		resp := Resp{SKU: sku, InStock: rand.Intn(2) == 0, LatencyMs: sleep}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	addr := ":8081"
	log.Println("inventory-service listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
