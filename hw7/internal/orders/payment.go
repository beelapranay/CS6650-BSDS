package orders

import (
	"context"
	"log/slog"
	"time"
)

type PaymentSimulator struct {
	name   string
	delay  time.Duration
	slots  chan struct{}
	logger *slog.Logger
}

func NewPaymentSimulator(name string, concurrency int, delay time.Duration, logger *slog.Logger) *PaymentSimulator {
	if concurrency < 1 {
		concurrency = 1
	}
	return &PaymentSimulator{
		name:   name,
		delay:  delay,
		slots:  make(chan struct{}, concurrency),
		logger: logger,
	}
}

func (p *PaymentSimulator) Process(ctx context.Context, order Order) error {
	queuedAt := time.Now()
	select {
	case p.slots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() {
		<-p.slots
	}()

	p.logger.InfoContext(
		ctx,
		"payment started",
		"component",
		p.name,
		"order_id",
		order.OrderID,
		"queued_for",
		time.Since(queuedAt).String(),
	)

	timer := time.NewTimer(p.delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		p.logger.InfoContext(
			ctx,
			"payment completed",
			"component",
			p.name,
			"order_id",
			order.OrderID,
			"processing_time",
			p.delay.String(),
		)
		return nil
	case <-ctx.Done():
		p.logger.WarnContext(ctx, "payment cancelled", "component", p.name, "order_id", order.OrderID)
		return ctx.Err()
	}
}
