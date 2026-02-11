package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stym06/rebuf/rebuf"
	"github.com/stym06/rebuf/server/internal/signing"
)

// Retry schedule: 5s, 5m, 30m, 2h, 5h, 10h, 10h
var retryDelays = []time.Duration{
	5 * time.Second,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	5 * time.Hour,
	10 * time.Hour,
	10 * time.Hour,
}

// DeliveryTask represents a single webhook delivery to perform.
type DeliveryTask struct {
	AttemptID  uuid.UUID
	MessageID  uuid.UUID
	EndpointID uuid.UUID
	URL        string
	Secret     string
	Payload    []byte
	EventType  string
}

// DeliveryWorker delivers webhooks to endpoints with retries.
type DeliveryWorker struct {
	db             *pgxpool.Pool
	wal            *rebuf.Rebuf
	client         *http.Client
	taskCh         chan DeliveryTask
	maxRetries     int
	numWorkers     int
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewDeliveryWorker creates a new delivery worker backed by a rebuf WAL.
func NewDeliveryWorker(db *pgxpool.Pool, wal *rebuf.Rebuf, numWorkers int, timeout time.Duration, maxRetries int) *DeliveryWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &DeliveryWorker{
		db:         db,
		wal:        wal,
		client:     &http.Client{Timeout: timeout},
		taskCh:     make(chan DeliveryTask, 10000),
		maxRetries: maxRetries,
		numWorkers: numWorkers,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start launches delivery worker goroutines and the retry poller.
func (w *DeliveryWorker) Start() {
	for i := 0; i < w.numWorkers; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}

	// Start retry poller — picks up failed attempts that are due for retry
	w.wg.Add(1)
	go w.retryPoller()

	slog.Info("delivery worker started", "workers", w.numWorkers)
}

// Stop gracefully shuts down the delivery workers.
func (w *DeliveryWorker) Stop() {
	w.cancel()
	close(w.taskCh)
	w.wg.Wait()
	slog.Info("delivery worker stopped")
}

// Enqueue adds a delivery task to the work queue.
// The task is also written to the WAL for durability.
func (w *DeliveryWorker) Enqueue(task DeliveryTask) {
	// Write to WAL for crash recovery
	data, err := json.Marshal(task)
	if err != nil {
		slog.Error("failed to marshal delivery task", "error", err)
		return
	}
	if err := w.wal.Write(data); err != nil {
		slog.Error("failed to write delivery task to WAL", "error", err)
	}

	// Send to worker channel (non-blocking)
	select {
	case w.taskCh <- task:
	default:
		slog.Warn("delivery task queue full, task will be picked up by retry poller",
			"attempt_id", task.AttemptID)
	}
}

func (w *DeliveryWorker) worker(id int) {
	defer w.wg.Done()

	for task := range w.taskCh {
		if w.ctx.Err() != nil {
			return
		}
		w.deliver(task)
	}
}

func (w *DeliveryWorker) deliver(task DeliveryTask) {
	now := time.Now()
	msgID := task.MessageID.String()

	// Sign the payload
	sig := signing.Sign(task.Secret, msgID, now, task.Payload)

	req, err := http.NewRequestWithContext(w.ctx, http.MethodPost, task.URL, bytes.NewReader(task.Payload))
	if err != nil {
		w.recordFailure(task, 0, fmt.Sprintf("failed to create request: %v", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Rebuf-Webhooks/1.0")
	req.Header.Set(signing.IDHeaderKey, msgID)
	req.Header.Set(signing.TimestampHeaderKey, fmt.Sprintf("%d", now.Unix()))
	req.Header.Set(signing.SignatureHeaderKey, sig)
	req.Header.Set("X-Rebuf-Event-Type", task.EventType)

	// Update status to sending
	w.db.Exec(w.ctx,
		`UPDATE message_attempts SET status = 'sending', updated_at = NOW() WHERE id = $1`,
		task.AttemptID,
	)

	resp, err := w.client.Do(req)
	if err != nil {
		w.recordFailure(task, 0, fmt.Sprintf("request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyStr := string(bodyBytes)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		w.recordSuccess(task, resp.StatusCode, bodyStr)
	} else {
		w.recordFailure(task, resp.StatusCode, bodyStr)
	}
}

func (w *DeliveryWorker) recordSuccess(task DeliveryTask, statusCode int, body string) {
	_, err := w.db.Exec(w.ctx,
		`UPDATE message_attempts
		 SET status = 'success', response_status_code = $2, response_body = $3, updated_at = NOW()
		 WHERE id = $1`,
		task.AttemptID, statusCode, truncate(body, 1024),
	)
	if err != nil {
		slog.Error("failed to record delivery success", "error", err, "attempt_id", task.AttemptID)
	}
}

func (w *DeliveryWorker) recordFailure(task DeliveryTask, statusCode int, body string) {
	// Get current attempt number
	var attemptNum int
	err := w.db.QueryRow(w.ctx,
		`SELECT attempt_number FROM message_attempts WHERE id = $1`,
		task.AttemptID,
	).Scan(&attemptNum)
	if err != nil {
		slog.Error("failed to get attempt number", "error", err, "attempt_id", task.AttemptID)
		return
	}

	if attemptNum >= w.maxRetries {
		// Max retries exceeded — mark as permanently failed
		w.db.Exec(w.ctx,
			`UPDATE message_attempts
			 SET status = 'failed', response_status_code = $2, response_body = $3, updated_at = NOW()
			 WHERE id = $1`,
			task.AttemptID, statusCode, truncate(body, 1024),
		)
		slog.Warn("webhook delivery permanently failed",
			"attempt_id", task.AttemptID, "endpoint", task.URL, "attempts", attemptNum)
		return
	}

	// Schedule retry with exponential backoff
	delayIdx := attemptNum - 1
	if delayIdx >= len(retryDelays) {
		delayIdx = len(retryDelays) - 1
	}
	nextRetry := time.Now().Add(retryDelays[delayIdx])

	var statusCodePtr *int
	if statusCode > 0 {
		statusCodePtr = &statusCode
	}

	_, err = w.db.Exec(w.ctx,
		`UPDATE message_attempts
		 SET status = 'pending',
		     response_status_code = $2,
		     response_body = $3,
		     attempt_number = attempt_number + 1,
		     next_retry_at = $4,
		     updated_at = NOW()
		 WHERE id = $1`,
		task.AttemptID, statusCodePtr, truncate(body, 1024), nextRetry,
	)
	if err != nil {
		slog.Error("failed to schedule retry", "error", err, "attempt_id", task.AttemptID)
	}
}

// retryPoller periodically picks up pending retries that are due.
func (w *DeliveryWorker) retryPoller() {
	defer w.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.pollRetries()
		}
	}
}

func (w *DeliveryWorker) pollRetries() {
	rows, err := w.db.Query(w.ctx,
		`SELECT ma.id, ma.message_id, ma.endpoint_id, e.url, e.secret, m.payload, m.event_type
		 FROM message_attempts ma
		 JOIN endpoints e ON ma.endpoint_id = e.id
		 JOIN messages m ON ma.message_id = m.id
		 WHERE ma.status = 'pending'
		   AND ma.next_retry_at IS NOT NULL
		   AND ma.next_retry_at <= NOW()
		   AND e.disabled = FALSE
		 ORDER BY ma.next_retry_at ASC
		 LIMIT 100`,
	)
	if err != nil {
		slog.Error("failed to poll retries", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var task DeliveryTask
		var payload json.RawMessage
		if err := rows.Scan(&task.AttemptID, &task.MessageID, &task.EndpointID,
			&task.URL, &task.Secret, &payload, &task.EventType); err != nil {
			slog.Error("failed to scan retry task", "error", err)
			continue
		}
		task.Payload, _ = json.Marshal(payload)

		select {
		case w.taskCh <- task:
		default:
			// Queue full, will be picked up next poll
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
