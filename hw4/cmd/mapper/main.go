package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"hw4/internal/s3util"
	"hw4/internal/textutil"

	"github.com/aws/aws-sdk-go-v2/config"
)

type outResp struct {
	Out string `json:"out"`
}

func main() {
	region := getenv("AWS_REGION", "us-west-2")
	port := getenv("PORT", "8080")

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/map", func(w http.ResponseWriter, r *http.Request) {
		chunkURL := r.URL.Query().Get("chunk")
		if chunkURL == "" {
			http.Error(w, "missing chunk param (s3://...)", http.StatusBadRequest)
			return
		}

		bucket, key, err := s3util.ParseS3URL(chunkURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		s3c := s3util.New(cfg, bucket)

		data, err := s3c.GetBytes(r.Context(), key)
		if err != nil {
			http.Error(w, "s3 get failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		counts := textutil.CountWords(data)

		js, err := json.Marshal(counts)
		if err != nil {
			http.Error(w, "json marshal failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		run := time.Now().Unix()
		outKey := fmt.Sprintf("maps/%d/%s.json", run, baseName(key))

		outURL, err := s3c.PutBytes(r.Context(), outKey, js, "application/json")
		if err != nil {
			http.Error(w, "s3 put failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(outResp{Out: outURL})
	})

	log.Printf("mapper listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func baseName(key string) string {
	// ".../chunk1.txt" -> "chunk1"
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '/' {
			key = key[i+1:]
			break
		}
	}
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '.' {
			return key[:i]
		}
	}
	return key
}
