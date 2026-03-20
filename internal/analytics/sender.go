package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Sender struct {
	cfg         Config
	client      *http.Client
	alertClient *http.Client
	queue       chan Record
	done        chan struct{}
	enabled     bool

	mu           sync.Mutex
	closed       bool
	outageActive bool
	outageSince  time.Time
	lostRecords  int
	lostBatches  int
	firstLossErr string

	alertsWG sync.WaitGroup
}

type retriableError struct {
	err error
}

func (e retriableError) Error() string {
	return e.err.Error()
}

func (e retriableError) Unwrap() error {
	return e.err
}

func NewSender(cfg Config) *Sender {
	cfg = cfg.normalized()

	sender := &Sender{
		cfg:         cfg,
		client:      &http.Client{Timeout: defaultHTTPTimeout},
		alertClient: &http.Client{Timeout: defaultAlertTimeout},
		enabled:     cfg.Enabled(),
	}

	if !sender.enabled {
		log.Println("analytics disabled: missing DIRECTUS_URL or DIRECTUS_SERVICE_TOKEN")
		return sender
	}

	sender.queue = make(chan Record, cfg.MaxQueueSize)
	sender.done = make(chan struct{})
	go sender.run()

	return sender
}

func (s *Sender) Enqueue(record Record) bool {
	if !s.enabled {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return false
	}

	select {
	case s.queue <- record:
		return true
	default:
	}

	select {
	case <-s.queue:
		s.recordLossLocked(1, 0, errors.New("analytics queue full"))
	default:
	}

	select {
	case s.queue <- record:
		return true
	default:
		s.recordLossLocked(1, 0, errors.New("analytics queue full"))
		return false
	}
}

func (s *Sender) Shutdown(ctx context.Context) error {
	if !s.enabled {
		return nil
	}

	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.queue)
	}
	done := s.done
	s.mu.Unlock()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	c := make(chan struct{})
	go func() {
		defer close(c)
		s.alertsWG.Wait()
	}()

	select {
	case <-c:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Sender) run() {
	defer close(s.done)

	ticker := time.NewTicker(s.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]Record, 0, s.cfg.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}

		records := append([]Record(nil), batch...)
		batch = batch[:0]

		if err := s.sendBatch(records); err != nil {
			log.Printf("analytics batch lost after retries: records=%d err=%v", len(records), err)
			s.recordLoss(len(records), 1, err)
			return
		}

		s.recordRecovery()
	}

	for {
		select {
		case record, ok := <-s.queue:
			if !ok {
				flush()
				return
			}

			batch = append(batch, record)
			if len(batch) >= s.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *Sender) sendBatch(records []Record) error {
	payload, err := json.Marshal(records)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 1; attempt <= defaultMaxAttempts; attempt++ {
		err = s.sendBatchOnce(payload)
		if err == nil {
			return nil
		}

		lastErr = err

		var retryable retriableError
		if !errors.As(err, &retryable) || attempt == defaultMaxAttempts {
			return lastErr
		}

		time.Sleep(defaultRetryBackoff)
	}

	return lastErr
}

func (s *Sender) sendBatchOnce(payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.DirectusURL+directusItemsEndpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+s.cfg.DirectusServiceToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return retriableError{err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	err = fmt.Errorf("directus returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return retriableError{err: err}
	}

	return err
}

func (s *Sender) recordLoss(recordsLost, batchesLost int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordLossLocked(recordsLost, batchesLost, err)
}

func (s *Sender) recordLossLocked(recordsLost, batchesLost int, err error) {
	s.lostRecords += recordsLost
	s.lostBatches += batchesLost

	if err != nil && s.firstLossErr == "" {
		s.firstLossErr = err.Error()
	}

	if s.outageActive {
		return
	}

	s.outageActive = true
	s.outageSince = time.Now().UTC()
	s.emitAlertLocked(formatLossAlert(s.outageSince, s.lostRecords, s.lostBatches, s.firstLossErr))
}

func (s *Sender) recordRecovery() {
	s.mu.Lock()
	if !s.outageActive {
		s.mu.Unlock()
		return
	}

	since := s.outageSince
	lostRecords := s.lostRecords
	lostBatches := s.lostBatches
	firstLossErr := s.firstLossErr

	s.outageActive = false
	s.outageSince = time.Time{}
	s.lostRecords = 0
	s.lostBatches = 0
	s.firstLossErr = ""
	s.mu.Unlock()

	recoveredAt := time.Now().UTC()
	message := fmt.Sprintf(
		"GS API analytics delivery recovered at %s.\n\nLoss window started at %s.\nLost records: %d.\nLost batches: %d.\nFirst error: %s.",
		recoveredAt.Format(time.RFC3339),
		since.Format(time.RFC3339),
		lostRecords,
		lostBatches,
		emptyFallback(firstLossErr, "n/a"),
	)
	s.emitAlert(message)
}

func (s *Sender) emitAlertLocked(message string) {
	s.emitAlert(message)
}

func (s *Sender) emitAlert(message string) {
	if strings.TrimSpace(s.cfg.TeamsWebhookURL) == "" {
		log.Printf("analytics alert: %s", message)
		return
	}

	s.alertsWG.Add(1)
	go func() {
		defer s.alertsWG.Done()
		if err := s.postTeamsAlert(message); err != nil {
			log.Printf("failed to send Teams analytics alert: %v", err)
		}
	}()
}

func (s *Sender) postTeamsAlert(message string) error {
	payload, err := json.Marshal(map[string]string{
		"@type":    "MessageCard",
		"@context": "http://schema.org/extensions",
		"summary":  "GS API analytics delivery alert",
		"title":    "GS API analytics delivery alert",
		"text":     message,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultAlertTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.TeamsWebhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.alertClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("teams webhook returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func formatLossAlert(at time.Time, lostRecords, lostBatches int, firstErr string) string {
	return fmt.Sprintf(
		"GS API analytics batch loss detected at %s.\n\nLost records: %d.\nLost batches: %d.\nFirst error: %s.",
		at.Format(time.RFC3339),
		lostRecords,
		lostBatches,
		emptyFallback(firstErr, "n/a"),
	)
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
