package kafkax

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
