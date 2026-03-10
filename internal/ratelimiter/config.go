package ratelimiter

import "time"

// Config содержит параметры rate limiting.
type Config struct {
	RequestsPerSecond float64       `yaml:"requests_per_second" env:"REQUESTS_PER_SECOND"`
	Burst             int           `yaml:"burst" env:"BURST"`
	CleanupInterval   time.Duration `yaml:"cleanup_interval" env:"CLEANUP_INTERVAL"`
}

// DefaultConfig возвращает конфигурацию по умолчанию.
func DefaultConfig() Config {
	return Config{
		RequestsPerSecond: 10.0,
		Burst:             20,
		CleanupInterval:   10 * time.Minute,
	}
}
