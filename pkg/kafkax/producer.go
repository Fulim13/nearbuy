package kafkax

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{writer: &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		WriteTimeout:           10 * time.Second,
		AllowAutoTopicCreation: false,
	}}
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

// Publish sends one message. key decides the partition, so it decides what is
// ordered with respect to what.

func (p *Producer) Publish(ctx context.Context, topic, key string, payload []byte, traceID string) error {
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
	}
	if traceID != "" {
		msg.Headers = append(msg.Headers, kafka.Header{
			Key: HeaderTraceID, Value: []byte(traceID),
		})
	}
	return p.writer.WriteMessages(ctx, msg)
}
