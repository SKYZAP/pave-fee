package fees

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

func (s *Service) runOutboxRelay(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := s.publishPendingEvents(ctx); err != nil {
			log.Printf("fees outbox relay failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) publishPendingEvents(ctx context.Context) error {
	events, err := s.repo.ListPendingOutbox(ctx, 100)
	if err != nil {
		return err
	}
	for _, event := range events {
		var invoice Invoice
		if err := json.Unmarshal(event.Payload, &invoice); err != nil {
			return err
		}
		if _, err := BillClosedTopic.Publish(ctx, &BillClosedEvent{
			EventID:          event.EventID.String(),
			BillID:           event.AggregateID.String(),
			AggregateVersion: event.AggregateVersion,
			EventType:        event.EventType,
			OccurredAt:       time.Now().UTC(),
			Invoice:          invoice,
		}); err != nil {
			return err
		}
		if err := s.repo.MarkOutboxPublished(ctx, event.EventID, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}
