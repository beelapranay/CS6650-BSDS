package s3util

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Client struct {
	Bucket   string
	uploader *manager.Uploader
	downloader *manager.Downloader
}

func New(cfg aws.Config, bucket string) *Client {
	s3c := s3.NewFromConfig(cfg)
	return &Client{
		Bucket:     bucket,
		uploader:   manager.NewUploader(s3c),
		downloader: manager.NewDownloader(s3c),
	}
}

func (c *Client) PutBytes(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	_, err := c.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("put s3://%s/%s: %w", c.Bucket, key, err)
	}
	return fmt.Sprintf("s3://%s/%s", c.Bucket, key), nil
}

func (c *Client) GetBytes(ctx context.Context, key string) ([]byte, error) {
	buf := manager.NewWriteAtBuffer([]byte{})
	_, err := c.downloader.Download(ctx, buf, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get s3://%s/%s: %w", c.Bucket, key, err)
	}
	return buf.Bytes(), nil
}

func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}