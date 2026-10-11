package kafkax

import (
	"context"

	"google.golang.org/grpc/metadata"
)

const HeaderTraceID = "trace-id"
const HeaderAttempt = "attempt" // delivery attempt

const (
	TopicSeckillAdmitted = "seckill.admitted"
	TopicOrderState      = "order.state_changed"
)

const DLQSuffix = ".dlq"

func DLQ(topic string) string {
	return topic + DLQSuffix
}

// Header2TraceID pulls the trace id out of an incoming gRPC context, so a
// producer can carry it onto the message. Returns "" when absent
// gateway need to always set it.
func Header2TraceID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if v := md.Get(HeaderTraceID); len(v) > 0 {
		return v[0]
	}
	return ""
}
