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

type resp struct {
	Chunks []string `json:"chunks"`
}

func main() {
	region := getenv("AWS_REGION", "us-west-2")
	bucket := getenv("BUCKET", "amzn-bucket-text-file-12345678")
	port := getenv("PORT", "8080")

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}
	s3c := s3util.New(cfg, bucket)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/split", func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Query().Get("url")
		if url == "" {
			http.Error(w, "missing url param", http.StatusBadRequest)
			return
		}

		// download text
		client := &http.Client{Timeout: 30 * time.Second}
		respHTTP, err := client.Get(url)
		if err != nil {
			http.Error(w, "download failed: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer respHTTP.Body.Close()
		if respHTTP.StatusCode != 200 {
			http.Error(w, fmt.Sprintf("download status %d", respHTTP.StatusCode), http.StatusBadRequest)
			return
		}

		data, err := s3util.ReadAll(respHTTP.Body)
		if err != nil {
			http.Error(w, "read failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// split into 3 chunks
		chunks := textutil.SplitIntoN(data, 3)

		// upload chunks
		ts := time.Now().Unix()
		outURLs := make([]string, 0, 3)
		ctx := r.Context()

		for i, c := range chunks {
			key := fmt.Sprintf("chunks/%d/chunk%d.txt", ts, i+1)
			u, err := s3c.PutBytes(ctx, key, c, "text/plain")
			if err != nil {
				http.Error(w, "s3 put failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			outURLs = append(outURLs, u)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp{Chunks: outURLs})
	})

	log.Printf("splitter listening on :%s (bucket=%s region=%s)", port, bucket, region)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}