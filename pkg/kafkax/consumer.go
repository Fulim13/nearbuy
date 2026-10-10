package kafkax

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

// Handler processes one message. Returning an error means "retry"; returning
// ErrPermanent means "do not retry, dead-letter it now".
type Handler func(ctx context.Context, m kafka.Message) error

// ErrPermanent marks a message that will never succeed -- malformed JSON, an
// unknown event version.
var ErrPermanent = errors.New("permanent failture")

type ConsumerConfig struct {
	Brokers     []string
	Topic       string
	GroupID     string
	MaxAttempts int // total attempts before the DLQ; 0 means 3
}

type Consumer struct {
	reader   *kafka.Reader
	producer *Producer
	cfg      ConsumerConfig
}

func NewConsumer(cfg ConsumerConfig, dlqProducer *Producer) *Consumer {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}

	return &Consumer{
		cfg:      cfg,
		producer: dlqProducer,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: cfg.Brokers,
			Topic:   cfg.Topic,
			GroupID: cfg.GroupID,
			// StartOffset applies only to a group with no committed offset yet.
			// FirstOffset: Reads all existing history, then new messages
			// LastOffset: Reads only messages produced from now on
			StartOffset: kafka.FirstOffset,
			MinBytes:    1,
			MaxBytes:    10e6,
			MaxWait:     500 * time.Millisecond,
		}),
	}
}

// Run consumes until ctx is cancelled.
//
// Offsets are committed MANUALLY, after the handler succeeds or after the message
// is dead-lettered.
func (c *Consumer) Run(ctx context.Context, h Handler) error {
	defer func() {
		if err := c.reader.Close(); err != nil {
			slog.Error("kafka reader close", "topic", c.cfg.Topic, "error", err)
		}
	}()

	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				slog.Info("consumer stopping", "topic", c.cfg.Topic, "group", c.cfg.GroupID)
				return nil
			}
			return fmt.Errorf("fetch: %w", err)
		}

		c.process(ctx, m, h)

		// Commit even for a dead-lettered message: the DLQ now owns it, and not
		// committing would replay it forever
		if err := c.reader.CommitMessages(ctx, m); err != nil && ctx.Err() == nil {
			slog.Error("commit failed", "topic", m.Topic,
				"partition", m.Partition, "offset", m.Offset, "error", err)
		}
	}
}

func (c *Consumer) process(ctx context.Context, m kafka.Message, h Handler) {
	traceID := Header(m, HeaderTraceID)
	log := slog.With("topic", m.Topic, "partition", m.Partition,
		"offset", m.Offset, "key", string(m.Key), "trace_id", traceID)

	attempt := attemptOf(m)
	for {
		attempt++
		err := h(ctx, m)
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			return // shutting down; the offset is uncommitted, so it redelivers
		}

		permanent := errors.Is(err, ErrPermanent)
		log.Warn("handler failed", "attempt", attempt,
			"permanent", permanent, "error", err)

		if permanent || attempt >= c.cfg.MaxAttempts {
			c.deadLetter(ctx, m, attempt, err)
			return
		}
		// Linear backoff. The wait grows each time: 200ms, 400ms, 600ms…
		// Deliberately short: a long sleep here stalls the whole
		// partition, because this consumer is single-threaded per partition.
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
		}
	}
}

// deadLetter forwards the message to <topic>.dlq with the failure recorded in its
// headers, so the original payload and the reason travel together.
func (c *Consumer) deadLetter(ctx context.Context, m kafka.Message, attempt int, cause error) {
	headers := []kafka.Header{
		{Key: HeaderAttempt, Value: []byte(fmt.Sprint(attempt))},
		{Key: "error", Value: []byte(cause.Error())},
		{Key: "origin-topic", Value: []byte(m.Topic)},
		{Key: "origin-partition", Value: []byte(fmt.Sprint(m.Partition))},
		{Key: "origin-offset", Value: []byte(fmt.Sprint(m.Offset))},
	}
	if t := Header(m, HeaderTraceID); t != "" {
		headers = append(headers, kafka.Header{Key: HeaderTraceID, Value: []byte(t)})
	}

	// A fresh context: ctx may already be cancelled, and dropping the message
	// here would lose it entirely.
	dlqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := c.producer.writer.WriteMessages(dlqCtx, kafka.Message{
		Topic:   DLQ(m.Topic),
		Key:     m.Key,
		Value:   m.Value,
		Headers: headers,
	})
	if err != nil {
		// Nothing left to try. Log loudly: this is the one path where a message
		// is genuinely lost.
		slog.Error("DEAD LETTER WRITE FAILED - message lost",
			"topic", m.Topic, "offset", m.Offset, "error", err)
		return
	}
	slog.Error("message dead-lettered", "topic", m.Topic, "dlq", DLQ(m.Topic),
		"offset", m.Offset, "attempts", attempt, "cause", cause)
}

// Header returns a header value, or "" when absent.
func Header(m kafka.Message, key string) string {
	for _, h := range m.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func attemptOf(m kafka.Message) int {
	var n int
	if v := Header(m, HeaderAttempt); v != "" {
		_, _ = fmt.Sscanf(v, "%d", &n)
	}
	return n
}
