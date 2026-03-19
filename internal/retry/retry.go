package retry

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

type Config struct {
	Attempts int
	Delay    time.Duration
	MaxDelay time.Duration
}

// Do retries fn up to cfg.Attempts times with exponential backoff.
// Only transient errors are retried; permanent errors stop immediately.
// If Attempts <= 1, fn is called once with no retry.
func Do(cfg Config, fn func() error) error {
	if cfg.Attempts <= 1 {
		return fn()
	}
	if cfg.Delay == 0 {
		cfg.Delay = time.Second
	}
	if cfg.MaxDelay == 0 {
		cfg.MaxDelay = 30 * time.Second
	}

	var lastErr error
	delay := cfg.Delay
	for i := range cfg.Attempts {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !isTransient(lastErr) {
			return lastErr
		}
		// Don't sleep after the last attempt
		if i < cfg.Attempts-1 {
			time.Sleep(delay)
			delay *= 2
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
		}
	}
	return fmt.Errorf("after %d attempts: %w", cfg.Attempts, lastErr)
}

func isTransient(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	// Connection reset/refused patterns
	msg := err.Error()
	for _, pattern := range []string{
		"connection reset",
		"connection refused",
		"broken pipe",
		"i/o timeout",
		"EOF",
	} {
		if strings.Contains(msg, pattern) {
			return true
		}
	}

	return false
}
