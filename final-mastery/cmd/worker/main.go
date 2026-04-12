package main

import (
	"log"
	"os"
	"strconv"

	"final-mastery/internal/albumstore"
)

func main() {
	log.SetFlags(log.LstdFlags)

	worker, err := albumstore.BuildAWSWorker(albumstore.MaxUploadBytesFromEnv())
	if err != nil {
		log.Fatalf("build worker: %v", err)
	}

	waitTime := envInt32("SQS_WAIT_TIME_SECONDS", 1)
	visibilityTimeout := envInt32("SQS_VISIBILITY_TIMEOUT", 60)

	if len(os.Args) > 1 && os.Args[1] == "--once" {
		_, err := worker.ProcessOnce(waitTime, visibilityTimeout)
		if err != nil {
			log.Fatalf("process once: %v", err)
		}
		return
	}

	if err := worker.RunForever(waitTime, visibilityTimeout); err != nil {
		log.Fatalf("run worker: %v", err)
	}
}

func envInt32(name string, fallback int32) int32 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return int32(parsed)
}
