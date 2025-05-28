// Package curb provides a generic retry mechanism with configurable backoff delays and jitter.
// It is useful for wrapping operations that may fail transiently, such as network or API calls.
package curb

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

var (
	// ErrInvalidMaxAttempts is returned when an invalid max attempt count is configured.
	ErrInvalidMaxAttempts = errors.New("curb: max attempts should be between 1 and 10")
)

// Retry retries the given function `fn` up to a configurable number of times,
// applying an exponential backoff with jitter between attempts.
// It stops early if the function succeeds or if the context is canceled.
// Returns the result of `fn`, or an error if all attempts fail or the context expires.
//
// Example usage:
//
//	res, err := curb.Retry(ctx, func() (Result, error) {
//	    return someUnstableOperation()
//	}, curb.WithMaxAttempts(3))
func Retry[T any](ctx context.Context, fn func() (T, error), opts ...option) (T, error) {
	var zero T
	cfg := config{
		maxAttempts: 5,
		sleeper:     time.After,
	}
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return zero, err
		}
	}
	for attempt := range cfg.maxAttempts {
		if res, err := fn(); err == nil {
			return res, nil
		}
		if attempt == cfg.maxAttempts-1 {
			return zero, ErrRetriesExhausted{MaxAttempts: cfg.maxAttempts}
		}

		var delay time.Duration
		if attempt < len(delays) {
			delay = delays[attempt]
		} else {
			delay = delays[len(delays)-1]
		}
		delay = calculateDelay(delay)

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-cfg.sleeper(delay):
			continue
		}
	}
	return zero, ErrRetriesExhausted{MaxAttempts: cfg.maxAttempts}
}

// ErrRetriesExhausted is returned when all retry attempts are exhausted.
type ErrRetriesExhausted struct {
	// MaxAttempts indicates how many times the function was retried.
	MaxAttempts int
}

// Error implements the error interface for ErrRetriesExhausted.
func (e ErrRetriesExhausted) Error() string {
	return fmt.Sprintf("curb: retries exhausted (max attempts %d)", e.MaxAttempts)
}

// option defines a functional option for configuring retry behavior.
type option func(cfg *config) error

type config struct {
	maxAttempts int
	sleeper     func(time.Duration) <-chan time.Time
}

// WithMaxAttempts sets the maximum number of retry attempts (must be between 1 and 10).
func WithMaxAttempts(m int) option {
	return func(cfg *config) error {
		if m < 1 || m > len(delays) {
			return ErrInvalidMaxAttempts
		}
		cfg.maxAttempts = m
		return nil
	}
}

// WithSleeper allows customization of the delay function used between retries.
//
// This option is primarily useful for testing, where you may want to avoid actual
// time delays by injecting a mocked or fake sleeper function. The provided sleeper
// function should return a channel that signals when to resume execution.
//
// Example usage:
//
//	fakeSleeper := func(d time.Duration) <-chan time.Time {
//	    ch := make(chan time.Time, 1)
//	    ch <- time.Now()
//	    return ch
//	}
//
//	curb.Retry(ctx, fn, curb.WithSleeper(fakeSleeper))
func WithSleeper(s func(time.Duration) <-chan time.Time) option {
	return func(cfg *config) error {
		cfg.sleeper = s
		return nil
	}
}

// delays defines a capped exponential backoff sequence.
var delays = []time.Duration{
	1 * time.Second, 2 * time.Second,
	4 * time.Second, 8 * time.Second,
	16 * time.Second, 32 * time.Second,
	60 * time.Second, 60 * time.Second,
	60 * time.Second, 60 * time.Second,
}

// calculateDelay adds jitter to a base delay value to avoid thundering herd problems.
func calculateDelay(delay time.Duration) time.Duration {
	jitterFactor := 0.5
	baseDelay := float64(delay) * 0.75
	jitter := rand.Float64() * float64(delay) * jitterFactor
	return time.Duration(baseDelay + jitter)
}
