package retry

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Config holds retry configuration.
type Config struct {
	MaxAttempts  int           // Maximum number of retry attempts (including the first try)
	InitialDelay time.Duration // Initial delay before first retry
	MaxDelay     time.Duration // Maximum delay between retries
	Multiplier   float64       // Exponential backoff multiplier
}

// DefaultConfig returns sensible defaults for retry behavior.
func DefaultConfig() Config {
	return Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

// Operation is a function that may fail and should be retried.
type Operation func() error

// OperationWithContext is a function that may fail and should be retried, with context support.
type OperationWithContext func(ctx context.Context) error

// Do executes an operation with exponential backoff retry logic.
// Returns the error from the last attempt if all retries are exhausted.
func Do(op Operation, cfg Config) error {
	return DoWithContext(context.Background(), func(ctx context.Context) error {
		return op()
	}, cfg)
}

// DoWithContext executes an operation with exponential backoff retry logic and context support.
// Returns the error from the last attempt if all retries are exhausted.
func DoWithContext(ctx context.Context, op OperationWithContext, cfg Config) error {
	var lastErr error
	
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}
		
		// Execute the operation
		lastErr = op(ctx)
		if lastErr == nil {
			return nil // Success!
		}
		
		// Don't retry if this was the last attempt
		if attempt >= cfg.MaxAttempts {
			break
		}
		
		// Calculate delay with exponential backoff
		delay := calculateDelay(attempt, cfg)
		
		// Wait before retrying (with context cancellation support)
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
		}
	}
	
	return fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// calculateDelay computes the exponential backoff delay for a given attempt.
func calculateDelay(attempt int, cfg Config) time.Duration {
	// Exponential backoff: initialDelay * (multiplier ^ (attempt - 1))
	delayFloat := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))
	delay := time.Duration(delayFloat)
	
	// Cap at maximum delay
	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}
	
	return delay
}

// IsRetryable checks if an error should trigger a retry.
// This can be extended with more sophisticated error analysis.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	
	// Add logic to determine if error is retryable
	// For now, we retry all errors except context cancellation
	errStr := err.Error()
	
	// Don't retry context cancellation
	if errStr == "context canceled" || errStr == "context deadline exceeded" {
		return false
	}
	
	// Retry network errors, timeouts, temporary errors
	// TODO: Add more sophisticated error type checking
	return true
}

// DoWithRetryable executes an operation with retry logic, but only retries if the error is retryable.
func DoWithRetryable(op Operation, cfg Config) error {
	return DoWithContextRetryable(context.Background(), func(ctx context.Context) error {
		return op()
	}, cfg)
}

// DoWithContextRetryable executes an operation with retry logic and context, only retrying retryable errors.
func DoWithContextRetryable(ctx context.Context, op OperationWithContext, cfg Config) error {
	var lastErr error
	
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}
		
		// Execute the operation
		lastErr = op(ctx)
		if lastErr == nil {
			return nil // Success!
		}
		
		// Check if error is retryable
		if !IsRetryable(lastErr) {
			return fmt.Errorf("non-retryable error: %w", lastErr)
		}
		
		// Don't retry if this was the last attempt
		if attempt >= cfg.MaxAttempts {
			break
		}
		
		// Calculate delay with exponential backoff
		delay := calculateDelay(attempt, cfg)
		
		// Wait before retrying (with context cancellation support)
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
		}
	}
	
	return fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}
