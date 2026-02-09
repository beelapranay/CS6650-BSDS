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
	port := getenv("PORT", "8082") // use 8082 so it doesn't clash

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/reduce", func(w http.ResponseWriter, r *http.Request) {
		m1 := r.URL.Query().Get("m1")
		m2 := r.URL.Query().Get("m2")
		m3 := r.URL.Query().Get("m3")
		if m1 == "" || m2 == "" || m3 == "" {
			http.Error(w, "missing m1/m2/m3 params (s3://...)", http.StatusBadRequest)
			return
		}

		merged := make(map[string]int)

		for _, u := range []string{m1, m2, m3} {
			bucket, key, err := s3util.ParseS3URL(u)
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

			part := make(map[string]int)
			if err := json.Unmarshal(data, &part); err != nil {
				http.Error(w, "json unmarshal failed: "+err.Error(), http.StatusInternalServerError)
				return
			}

			textutil.MergeCounts(merged, part)
		}

		js, err := json.Marshal(merged)
		if err != nil {
			http.Error(w, "json marshal failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// write final result into the bucket of m1 (same bucket in your case)
		bucket, _, _ := s3util.ParseS3URL(m1)
		s3c := s3util.New(cfg, bucket)

		run := time.Now().Unix()
		outKey := fmt.Sprintf("reduce/%d/final.json", run)

		outURL, err := s3c.PutBytes(r.Context(), outKey, js, "application/json")
		if err != nil {
			http.Error(w, "s3 put failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(outResp{Out: outURL})
	})

	log.Printf("reducer listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
