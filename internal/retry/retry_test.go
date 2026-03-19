package retry

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func TestDoSuccessFirst(t *testing.T) {
	calls := 0
	err := Do(Config{Attempts: 3, Delay: time.Millisecond}, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDoSuccessNth(t *testing.T) {
	calls := 0
	err := Do(Config{Attempts: 5, Delay: time.Millisecond}, func() error {
		calls++
		if calls < 3 {
			return fmt.Errorf("connection reset")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDoAllFail(t *testing.T) {
	calls := 0
	err := Do(Config{Attempts: 3, Delay: time.Millisecond}, func() error {
		calls++
		return fmt.Errorf("i/o timeout")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDoNonTransientStopsImmediately(t *testing.T) {
	calls := 0
	err := Do(Config{Attempts: 5, Delay: time.Millisecond}, func() error {
		calls++
		return fmt.Errorf("authentication failed")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (non-transient stops immediately), got %d", calls)
	}
}

func TestDoNoRetry(t *testing.T) {
	calls := 0
	err := Do(Config{Attempts: 0}, func() error {
		calls++
		return fmt.Errorf("some error")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDoBackoff(t *testing.T) {
	var timestamps []time.Time
	err := Do(Config{Attempts: 3, Delay: 50 * time.Millisecond, MaxDelay: 200 * time.Millisecond}, func() error {
		timestamps = append(timestamps, time.Now())
		return fmt.Errorf("connection refused")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(timestamps) != 3 {
		t.Fatalf("expected 3 timestamps, got %d", len(timestamps))
	}
	// First delay should be ~50ms, second ~100ms
	d1 := timestamps[1].Sub(timestamps[0])
	d2 := timestamps[2].Sub(timestamps[1])
	if d1 < 30*time.Millisecond || d1 > 100*time.Millisecond {
		t.Errorf("first delay %v not in expected range [30ms, 100ms]", d1)
	}
	if d2 < 60*time.Millisecond || d2 > 200*time.Millisecond {
		t.Errorf("second delay %v not in expected range [60ms, 200ms]", d2)
	}
}

// timeoutError implements net.Error for testing
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "operation timed out" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

var _ net.Error = (*timeoutError)(nil)

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"timeout", &timeoutError{}, true},
		{"connection reset", fmt.Errorf("connection reset by peer"), true},
		{"connection refused", fmt.Errorf("connection refused"), true},
		{"broken pipe", fmt.Errorf("write: broken pipe"), true},
		{"io timeout", fmt.Errorf("i/o timeout"), true},
		{"EOF", fmt.Errorf("unexpected EOF"), true},
		{"auth failure", fmt.Errorf("authentication failed"), false},
		{"not found", fmt.Errorf("dataset not found"), false},
		{"wrapped transient", fmt.Errorf("connect: %w", fmt.Errorf("connection refused")), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransient(tt.err); got != tt.want {
				t.Errorf("isTransient(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
