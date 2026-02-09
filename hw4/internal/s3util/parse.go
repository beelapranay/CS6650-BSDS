package s3util

import (
	"fmt"
	"strings"
)

func ParseS3URL(s string) (bucket, key string, err error) {
	if !strings.HasPrefix(s, "s3://") {
		return "", "", fmt.Errorf("not an s3 url: %s", s)
	}
	trim := strings.TrimPrefix(s, "s3://")
	parts := strings.SplitN(trim, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("bad s3 url: %s", s)
	}
	return parts[0], parts[1], nil
}