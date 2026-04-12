package albumstore

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type awsAlbumRepository struct {
	client    *dynamodb.Client
	tableName string
}

type albumItem struct {
	AlbumID     string `dynamodbav:"album_id"`
	Title       string `dynamodbav:"title"`
	Description string `dynamodbav:"description"`
	Owner       string `dynamodbav:"owner"`
	CreatedAt   string `dynamodbav:"created_at"`
	UpdatedAt   string `dynamodbav:"updated_at"`
}

func (r *awsAlbumRepository) Upsert(album Album) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.client.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]ddbtypes.AttributeValue{
			"album_id": &ddbtypes.AttributeValueMemberS{Value: album.AlbumID},
		},
		UpdateExpression: aws.String("SET title = :title, description = :description, #o = :owner, updated_at = :updated_at, created_at = if_not_exists(created_at, :now)"),
		ExpressionAttributeNames: map[string]string{
			"#o": "owner",
		},
		ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
			":title":       &ddbtypes.AttributeValueMemberS{Value: album.Title},
			":description": &ddbtypes.AttributeValueMemberS{Value: album.Description},
			":owner":       &ddbtypes.AttributeValueMemberS{Value: album.Owner},
			":updated_at":  &ddbtypes.AttributeValueMemberS{Value: now},
			":now":         &ddbtypes.AttributeValueMemberS{Value: now},
		},
		ReturnValues: ddbtypes.ReturnValueAllOld,
	})
	if err != nil {
		return false, err
	}
	return len(result.Attributes) == 0, nil
}

func (r *awsAlbumRepository) Get(albumID string) (*Album, error) {
	ctx := context.Background()
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]ddbtypes.AttributeValue{
			"album_id": &ddbtypes.AttributeValueMemberS{Value: albumID},
		},
	})
	if err != nil || result.Item == nil {
		return nil, err
	}
	var item albumItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, err
	}
	return &Album{
		AlbumID:     item.AlbumID,
		Title:       item.Title,
		Description: item.Description,
		Owner:       item.Owner,
	}, nil
}

func (r *awsAlbumRepository) ListAll() ([]Album, error) {
	ctx := context.Background()
	var albums []Album
	var startKey map[string]ddbtypes.AttributeValue
	for {
		result, err := r.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(r.tableName),
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, err
		}
		var items []albumItem
		if err := attributevalue.UnmarshalListOfMaps(result.Items, &items); err != nil {
			return nil, err
		}
		for _, item := range items {
			albums = append(albums, Album{
				AlbumID:     item.AlbumID,
				Title:       item.Title,
				Description: item.Description,
				Owner:       item.Owner,
			})
		}
		if len(result.LastEvaluatedKey) == 0 {
			break
		}
		startKey = result.LastEvaluatedKey
	}
	return albums, nil
}

func (r *awsAlbumRepository) Exists(albumID string) (bool, error) {
	ctx := context.Background()
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]ddbtypes.AttributeValue{
			"album_id": &ddbtypes.AttributeValueMemberS{Value: albumID},
		},
		ProjectionExpression: aws.String("album_id"),
	})
	if err != nil {
		return false, err
	}
	return result.Item != nil, nil
}

type awsPhotoRepository struct {
	client           *dynamodb.Client
	tableName        string
	counterTableName string
}

type photoItem struct {
	PhotoID          string `dynamodbav:"photo_id"`
	AlbumID          string `dynamodbav:"album_id"`
	Seq              int64  `dynamodbav:"seq"`
	Status           string `dynamodbav:"status"`
	OriginalFilename string `dynamodbav:"original_filename"`
	TempPath         string `dynamodbav:"temp_path"`
	StoragePath      string `dynamodbav:"storage_path"`
	CreatedAt        string `dynamodbav:"created_at"`
	UpdatedAt        string `dynamodbav:"updated_at"`
}

func (r *awsPhotoRepository) AllocateSeq(albumID string) (int64, error) {
	ctx := context.Background()
	out, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.counterTableName),
		Key: map[string]ddbtypes.AttributeValue{
			"album_id": &ddbtypes.AttributeValueMemberS{Value: albumID},
		},
		UpdateExpression: aws.String("SET current_seq = if_not_exists(current_seq, :zero) + :one"),
		ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
			":zero": &ddbtypes.AttributeValueMemberN{Value: "0"},
			":one":  &ddbtypes.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: ddbtypes.ReturnValueUpdatedNew,
	})
	if err != nil {
		return 0, err
	}
	value := out.Attributes["current_seq"].(*ddbtypes.AttributeValueMemberN).Value
	return strconv.ParseInt(value, 10, 64)
}

func (r *awsPhotoRepository) CreateProcessingPhoto(photoID, albumID string, seq int64, stagedUpload StagedUpload) error {
	item, err := attributevalue.MarshalMap(photoItem{
		PhotoID:          photoID,
		AlbumID:          albumID,
		Seq:              seq,
		Status:           "processing",
		OriginalFilename: stagedUpload.OriginalFilename,
		TempPath:         stagedUpload.TempPath,
		StoragePath:      "",
		CreatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	_, err = r.client.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

func (r *awsPhotoRepository) Get(albumID, photoID string) (*PhotoRecord, error) {
	record, err := r.GetByPhotoID(photoID)
	if err != nil || record == nil || record.AlbumID != albumID {
		if record != nil && record.AlbumID != albumID {
			return nil, nil
		}
		return record, err
	}
	return record, nil
}

func (r *awsPhotoRepository) GetByPhotoID(photoID string) (*PhotoRecord, error) {
	result, err := r.client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]ddbtypes.AttributeValue{
			"photo_id": &ddbtypes.AttributeValueMemberS{Value: photoID},
		},
	})
	if err != nil || result.Item == nil {
		return nil, err
	}
	var item photoItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, err
	}
	return &PhotoRecord{
		PhotoID:          item.PhotoID,
		AlbumID:          item.AlbumID,
		Seq:              item.Seq,
		Status:           item.Status,
		OriginalFilename: item.OriginalFilename,
		TempPath:         item.TempPath,
		StoragePath:      item.StoragePath,
	}, nil
}

func (r *awsPhotoRepository) MarkCompleted(albumID, photoID, storagePath string) (bool, error) {
	_, err := r.client.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]ddbtypes.AttributeValue{
			"photo_id": &ddbtypes.AttributeValueMemberS{Value: photoID},
		},
		ConditionExpression: aws.String("attribute_exists(photo_id) AND album_id = :album_id"),
		UpdateExpression:    aws.String("SET #status = :status, temp_path = :temp_path, storage_path = :storage_path, updated_at = :updated_at"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
			":album_id":     &ddbtypes.AttributeValueMemberS{Value: albumID},
			":status":       &ddbtypes.AttributeValueMemberS{Value: "completed"},
			":temp_path":    &ddbtypes.AttributeValueMemberS{Value: ""},
			":storage_path": &ddbtypes.AttributeValueMemberS{Value: storagePath},
			":updated_at":   &ddbtypes.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339Nano)},
		},
	})
	if err == nil {
		return true, nil
	}
	var conditional *ddbtypes.ConditionalCheckFailedException
	if errors.As(err, &conditional) {
		return false, nil
	}
	return false, err
}

func (r *awsPhotoRepository) MarkFailed(albumID, photoID string) error {
	_, err := r.client.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]ddbtypes.AttributeValue{
			"photo_id": &ddbtypes.AttributeValueMemberS{Value: photoID},
		},
		ConditionExpression: aws.String("attribute_exists(photo_id) AND album_id = :album_id"),
		UpdateExpression:    aws.String("SET #status = :status, updated_at = :updated_at"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
			":album_id":   &ddbtypes.AttributeValueMemberS{Value: albumID},
			":status":     &ddbtypes.AttributeValueMemberS{Value: "failed"},
			":updated_at": &ddbtypes.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339Nano)},
		},
	})
	var conditional *ddbtypes.ConditionalCheckFailedException
	if errors.As(err, &conditional) {
		return nil
	}
	return err
}

func (r *awsPhotoRepository) Delete(albumID, photoID string) (*PhotoRecord, error) {
	record, err := r.Get(albumID, photoID)
	if err != nil || record == nil {
		return record, err
	}
	_, err = r.client.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]ddbtypes.AttributeValue{
			"photo_id": &ddbtypes.AttributeValueMemberS{Value: photoID},
		},
	})
	return record, err
}

type awsFileStorage struct {
	s3Client      *s3.Client
	uploader      *manager.Uploader
	bucketName    string
	mediaPrefix   string
	region        string
	publicBaseURL string
}

func newAWSFileStorage(s3Client *s3.Client, bucketName, mediaPrefix, region, publicBaseURL string) *awsFileStorage {
	return &awsFileStorage{
		s3Client: s3Client,
		uploader: manager.NewUploader(s3Client, func(u *manager.Uploader) {
			u.Concurrency = 10
		}),
		bucketName:    bucketName,
		mediaPrefix:   mediaPrefix,
		region:        region,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}
}

func (s *awsFileStorage) StageUpload(file io.Reader, photoID, originalFilename string, maxUploadBytes int64) (StagedUpload, error) {
	ext := trimExtension(originalFilename)
	key := joinS3Key(s.mediaPrefix, photoID+ext)
	limited := &countingReader{reader: file, maxBytes: maxUploadBytes}
	_, err := s.uploader.Upload(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        limited,
		ContentType: aws.String(contentTypeForName(originalFilename)),
	})
	if err != nil {
		if errors.Is(err, errPayloadTooLarge) {
			return StagedUpload{}, err
		}
		return StagedUpload{}, err
	}
	return StagedUpload{OriginalFilename: originalFilename, TempPath: key}, nil
}

func (s *awsFileStorage) Promote(stagedUpload StagedUpload, photoID string) (string, error) {
	return stagedUpload.TempPath, nil
}

func (s *awsFileStorage) DeleteFile(path string) error {
	if path == "" {
		return nil
	}
	_, err := s.s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(path),
	})
	return err
}

func (s *awsFileStorage) StatStoredFile(path string) (*StoredFile, error) {
	out, err := s.s3Client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, nil
	}
	return &StoredFile{
		Path:          path,
		ContentType:   aws.ToString(out.ContentType),
		ContentLength: aws.ToInt64(out.ContentLength),
	}, nil
}

func (s *awsFileStorage) OpenStoredFile(path string) (io.ReadCloser, *StoredFile, error) {
	out, err := s.s3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, nil, err
	}
	return out.Body, &StoredFile{
		Path:          path,
		ContentType:   aws.ToString(out.ContentType),
		ContentLength: aws.ToInt64(out.ContentLength),
	}, nil
}

func (s *awsFileStorage) BuildPublicURL(storagePath, baseURL, photoID string) string {
	if s.publicBaseURL != "" {
		return s.publicBaseURL + "/" + storagePath
	}
	if s.region != "" {
		return "https://" + s.bucketName + ".s3." + s.region + ".amazonaws.com/" + storagePath
	}
	return "https://" + s.bucketName + ".s3.amazonaws.com/" + storagePath
}

type countingReader struct {
	reader   io.Reader
	read     int64
	maxBytes int64
}

var errPayloadTooLarge = errors.New("payload too large")

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += int64(n)
	if r.read > r.maxBytes {
		return n, errPayloadTooLarge
	}
	return n, err
}

type sqsDispatcher struct {
	client   *sqs.Client
	queueURL string
}

func (d *sqsDispatcher) EnqueuePhotoProcessing(albumID, photoID string) error {
	payload, err := json.Marshal(map[string]string{
		"album_id": albumID,
		"photo_id": photoID,
	})
	if err != nil {
		return err
	}
	_, err = d.client.SendMessage(context.Background(), &sqs.SendMessageInput{
		QueueUrl:    aws.String(d.queueURL),
		MessageBody: aws.String(string(payload)),
	})
	return err
}

func (d *sqsDispatcher) Close() error {
	return nil
}

type inlineDispatcher struct {
	processFunc func(albumID, photoID string)
}

func (d *inlineDispatcher) EnqueuePhotoProcessing(albumID, photoID string) error {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("inline photo processing panic: %v", r)
			}
		}()
		d.processFunc(albumID, photoID)
	}()
	return nil
}

func (d *inlineDispatcher) Close() error {
	return nil
}

type Worker struct {
	client     *sqs.Client
	queueURL   string
	service    *Service
	maxWorkers int
}

func (w *Worker) ProcessOnce(waitTimeSeconds, visibilityTimeout int32) (bool, error) {
	result, err := w.client.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(w.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     waitTimeSeconds,
		VisibilityTimeout:   visibilityTimeout,
	})
	if err != nil {
		return false, err
	}
	if len(result.Messages) == 0 {
		return false, nil
	}

	sem := make(chan struct{}, w.maxWorkers)
	var wg sync.WaitGroup
	for _, message := range result.Messages {
		msg := message
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := w.processMessage(msg); err != nil {
				log.Printf("worker failed err=%v", err)
			}
		}()
	}
	wg.Wait()
	return true, nil
}

func (w *Worker) RunForever(waitTimeSeconds, visibilityTimeout int32) error {
	for {
		if _, err := w.ProcessOnce(waitTimeSeconds, visibilityTimeout); err != nil {
			return err
		}
	}
}

func (w *Worker) processMessage(message sqstypes.Message) error {
	var payload struct {
		AlbumID string `json:"album_id"`
		PhotoID string `json:"photo_id"`
	}
	if err := json.Unmarshal([]byte(aws.ToString(message.Body)), &payload); err != nil {
		return err
	}
	w.service.ProcessPhoto(payload.AlbumID, payload.PhotoID)
	_, err := w.client.DeleteMessage(context.Background(), &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(w.queueURL),
		ReceiptHandle: message.ReceiptHandle,
	})
	return err
}

func joinS3Key(prefix, name string) string {
	cleaned := strings.Trim(prefix, "/")
	if cleaned == "" {
		return name
	}
	return cleaned + "/" + name
}

func contentTypeForName(name string) string {
	if contentType := mime.TypeByExtension(filepath.Ext(name)); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func BuildAWSService(maxUploadBytes int64) (*Service, error) {
	cfg, err := loadAWSConfig()
	if err != nil {
		return nil, err
	}
	albumsTable, err := requiredEnv("ALBUMS_TABLE_NAME")
	if err != nil {
		return nil, err
	}
	photosTable, err := requiredEnv("PHOTOS_TABLE_NAME")
	if err != nil {
		return nil, err
	}
	countersTable, err := requiredEnv("PHOTO_COUNTERS_TABLE_NAME")
	if err != nil {
		return nil, err
	}
	bucketName, err := requiredEnv("PHOTO_BUCKET_NAME")
	if err != nil {
		return nil, err
	}
	// PHOTO_QUEUE_URL still required for worker builds but unused by inline dispatcher
	if _, err := requiredEnv("PHOTO_QUEUE_URL"); err != nil {
		return nil, err
	}

	ddbClient := dynamodb.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)

	dispatcher := &inlineDispatcher{}
	service := NewService(
		&awsAlbumRepository{client: ddbClient, tableName: albumsTable},
		&awsPhotoRepository{
			client:           ddbClient,
			tableName:        photosTable,
			counterTableName: countersTable,
		},
		newAWSFileStorage(
			s3Client,
			bucketName,
			envOrDefault("PHOTO_MEDIA_PREFIX", "media"),
			cfg.Region,
			os.Getenv("S3_PUBLIC_BASE_URL"),
		),
		dispatcher,
		maxUploadBytes,
	)
	dispatcher.processFunc = service.ProcessPhoto
	return service, nil
}

func BuildAWSWorker(maxUploadBytes int64) (*Worker, error) {
	service, err := BuildAWSService(maxUploadBytes)
	if err != nil {
		return nil, err
	}
	cfg, err := loadAWSConfig()
	if err != nil {
		return nil, err
	}
	queueURL, err := requiredEnv("PHOTO_QUEUE_URL")
	if err != nil {
		return nil, err
	}
	return &Worker{
		client:     sqs.NewFromConfig(cfg),
		queueURL:   queueURL,
		service:    service,
		maxWorkers: envInt("WORKER_THREADS", 20),
	}, nil
}

func loadAWSConfig() (aws.Config, error) {
	region := envOrDefault("AWS_REGION", os.Getenv("AWS_DEFAULT_REGION"))
	if region == "" {
		return aws.Config{}, errors.New("missing required environment variable: AWS_REGION")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 500
	transport.MaxConnsPerHost = 500
	transport.MaxIdleConnsPerHost = 500
	transport.ForceAttemptHTTP2 = true
	httpClient := &http.Client{Transport: transport}
	return awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithHTTPClient(httpClient),
	)
}

func requiredEnv(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", errors.New("missing required environment variable: " + name)
	}
	return value, nil
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
