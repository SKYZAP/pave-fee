package fees

import (
	"context"
	"time"

	"encore.app/fees/config"
	"encore.dev/pubsub"
	"encore.dev/storage/sqldb"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

//encore:service
type Service struct {
	repo      *Repository
	temporal  client.Client
	worker    worker.Worker
	stopRelay context.CancelFunc
}

var billingDB = sqldb.NewDatabase("fees", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

func initService() (*Service, error) {
	cfg := config.Load()
	temporalClient, err := client.Dial(client.Options{
		HostPort:  cfg.TemporalAddress,
		Namespace: cfg.TemporalNamespace,
	})
	if err != nil {
		return nil, err
	}

	repo := &Repository{DB: billingDB}
	billingWorker := worker.New(temporalClient, cfg.TemporalTaskQueue, worker.Options{})
	billingWorker.RegisterWorkflow(BillWorkflow)
	activities := &Activities{Repository: repo}
	billingWorker.RegisterActivity(activities)
	if err := billingWorker.Start(); err != nil {
		temporalClient.Close()
		return nil, err
	}

	relayCtx, cancel := context.WithCancel(context.Background())
	service := &Service{
		repo:      repo,
		temporal:  temporalClient,
		worker:    billingWorker,
		stopRelay: cancel,
	}
	go service.runOutboxRelay(relayCtx)
	return service, nil
}

func (s *Service) Shutdown(force context.Context) {
	s.stopRelay()
	s.worker.Stop()
	s.temporal.Close()
}

type BillClosedEvent struct {
	EventID          string    `json:"event_id"`
	BillID           string    `json:"bill_id"`
	AggregateVersion int64     `json:"aggregate_version"`
	EventType        string    `json:"event_type"`
	OccurredAt       time.Time `json:"occurred_at"`
	Invoice          Invoice   `json:"invoice"`
}

var BillClosedTopic = pubsub.NewTopic[*BillClosedEvent]("bill-closed", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})
