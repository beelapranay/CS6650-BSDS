package albumstore

import (
	"os"
	"strings"
)

func BuildServiceFromEnv(dataDir string, maxUploadBytes int64) (*Service, error) {
	switch strings.ToLower(strings.TrimSpace(envOrDefault("ALBUM_STORE_BACKEND", "local"))) {
	case "", "local":
		return BuildLocalService(dataDir, maxUploadBytes)
	case "aws":
		return BuildAWSService(maxUploadBytes)
	default:
		return nil, NewAppError(500, map[string]any{"error": "unsupported backend"})
	}
}

func PublicBaseURLFromEnv() string {
	return strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/")
}
