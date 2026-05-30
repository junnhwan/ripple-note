package outbox

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Publisher interface {
	Publish(topic string, payload []byte) error
}

type NopPublisher struct{}

func (n *NopPublisher) Publish(_ string, _ []byte) error { return nil }

type Worker struct {
	repo      *Repository
	publisher Publisher
	logger    *slog.Logger
	interval  time.Duration
	batchSize int

	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewWorker(repo *Repository, publisher Publisher, logger *slog.Logger, interval time.Duration, batchSize int) *Worker {
	if interval == 0 {
		interval = 5 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &Worker{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
		interval:  interval,
		batchSize: batchSize,
		stopCh:    make(chan struct{}),
	}
}

func (w *Worker) Start() {
	w.wg.Add(1)
	go w.run()
}

func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *Worker) run() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			w.logger.Info("outbox worker stopped")
			return
		case <-ticker.C:
			w.processBatch()
		}
	}
}

func (w *Worker) processBatch() {
	ctx := context.Background()
	events, err := w.repo.FindPending(ctx, w.batchSize)
	if err != nil {
		w.logger.Error("outbox: find pending events failed", "error", err)
		return
	}
	if len(events) == 0 {
		return
	}

	var sentIDs []uint64
	for _, event := range events {
		if err := w.publisher.Publish(event.Topic, []byte(event.Payload)); err != nil {
			w.logger.Error("outbox: publish event failed",
				"event_id", event.ID,
				"topic", event.Topic,
				"error", err,
			)
			nextRetry := time.Now().Add(30 * time.Second)
			_ = w.repo.MarkFailed(ctx, event.ID, event.RetryCount+1, nextRetry)
			continue
		}
		sentIDs = append(sentIDs, event.ID)
	}

	if len(sentIDs) > 0 {
		if err := w.repo.MarkSent(ctx, sentIDs); err != nil {
			w.logger.Error("outbox: mark sent failed", "error", err)
		}
		w.logger.Info("outbox: published events", "count", len(sentIDs))
	}
}
