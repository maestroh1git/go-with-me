package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Publisher publishes domain events to a Redis Stream.
type Publisher struct {
	rdb    *redis.Client
	stream string
}

// NewPublisher creates a Publisher that writes to the given Redis stream.
func NewPublisher(rdb *redis.Client, stream string) *Publisher {
	return &Publisher{rdb: rdb, stream: stream}
}

// Publish serialises the event payload and appends it to the stream via XADD.
func (p *Publisher) Publish(ctx context.Context, event Event) error {
	data, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: p.stream,
		Values: map[string]any{
			"event_id":     event.ID,
			"event_type":   event.Type,
			"payload":      string(data),
			"occurred_at":  event.OccurredAt.UTC().Format(time.RFC3339Nano),
		},
	}).Err()
}

// PublishRaw is a convenience helper that wraps any payload in an Event and publishes it.
func (p *Publisher) PublishRaw(ctx context.Context, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return p.Publish(ctx, Event{
		Type:       eventType,
		Payload:    json.RawMessage(data),
		OccurredAt: time.Now().UTC(),
	})
}
