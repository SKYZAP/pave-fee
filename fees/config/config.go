package config

import (
	"os"
)

type Config struct {
	TemporalAddress   string
	TemporalNamespace string
	TemporalTaskQueue string
}

func Load() Config {
	return Config{
		TemporalAddress:   envOr("FEES_TEMPORAL_ADDRESS", "127.0.0.1:7233"),
		TemporalNamespace: envOr("FEES_TEMPORAL_NAMESPACE", "default"),
		TemporalTaskQueue: envOr("FEES_TEMPORAL_TASK_QUEUE", "fees-bills"),
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
