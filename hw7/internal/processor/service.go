package processor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"hw7/internal/orders"
)

type Service struct {
	batchSize         int32
	client            *sqs.Client
	logger            *slog.Logger
	payments          *orders.PaymentSimulator
	queueURL          string
	visibilityTimeout int32
	waitTimeSeconds   int32
	workerCount       int
}

func New(ctx context.Context, logger *slog.Logger, queueURL string, workerCount int, delay time.Duration) (*Service, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &Service{
		batchSize:         10,
		client:            sqs.NewFromConfig(cfg),
		logger:            logger,
		payments:          orders.NewPaymentSimulator("order-processor", workerCount, delay, logger),
		queueURL:          queueURL,
		visibilityTimeout: 30,
		waitTimeSeconds:   20,
		workerCount:       workerCount,
	}, nil
}

func (s *Service) Run(ctx context.Context) error {
	jobs := make(chan sqstypes.Message, s.workerCount*2)
	var wg sync.WaitGroup

	for i := 0; i < s.workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			s.workerLoop(ctx, workerID+1, jobs)
		}(i)
	}

	defer func() {
		close(jobs)
		wg.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		output, err := s.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			MaxNumberOfMessages:   s.batchSize,
			QueueUrl:              &s.queueURL,
			VisibilityTimeout:     s.visibilityTimeout,
			WaitTimeSeconds:       s.waitTimeSeconds,
			MessageAttributeNames: []string{"All"},
		})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger.ErrorContext(ctx, "failed to receive SQS messages", "error", err)
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}

		for _, message := range output.Messages {
			select {
			case jobs <- message:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func (s *Service) workerLoop(ctx context.Context, workerID int, jobs <-chan sqstypes.Message) {
	for message := range jobs {
		if err := s.handleMessage(ctx, workerID, message); err != nil {
			s.logger.ErrorContext(ctx, "failed to process order message", "worker_id", workerID, "error", err)
		}
	}
}

func (s *Service) handleMessage(ctx context.Context, workerID int, message sqstypes.Message) error {
	order, err := DecodeOrderMessage(value(message.Body))
	if err != nil {
		s.logger.WarnContext(ctx, "dropping invalid order message", "worker_id", workerID, "error", err)
		return s.deleteMessage(ctx, message)
	}

	order.Status = orders.StatusProcessing
	s.logger.InfoContext(ctx, "worker received order", "worker_id", workerID, "order_id", order.OrderID)

	if err := s.payments.Process(ctx, order); err != nil {
		return err
	}

	order.Status = orders.StatusCompleted
	s.logger.InfoContext(ctx, "worker completed order", "worker_id", workerID, "order_id", order.OrderID)
	return s.deleteMessage(ctx, message)
}

func (s *Service) deleteMessage(ctx context.Context, message sqstypes.Message) error {
	_, err := s.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &s.queueURL,
		ReceiptHandle: message.ReceiptHandle,
	})
	return err
}

func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
