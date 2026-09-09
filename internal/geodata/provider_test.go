package geodata

import (
	"context"
	"errors"
	"testing"
	"time"
)

func retryRequestTestContext(
	retries int,
	onRetry func(attempt, total int, delay time.Duration, err error),
) context.Context {
	return context.WithValue(context.Background(), providerRetryContextKey{}, providerRetryConfig{
		retries:    retries,
		retryDelay: time.Millisecond,
		onRetry:    onRetry,
	})
}

func TestRetryProviderRequestSucceedsAfterTransientFailures(t *testing.T) {
	attempts := 0
	var retries []int
	ctx := retryRequestTestContext(
		2,
		func(attempt, total int, _ time.Duration, _ error) {
			if total != 2 {
				t.Fatalf("retry total = %d", total)
			}
			retries = append(retries, attempt)
		},
	)
	err := retryProviderRequest(
		ctx,
		func() error {
			attempts++
			if attempts < 3 {
				return markProviderErrorRetryable(errors.New("TLS handshake timeout"))
			}
			return nil
		},
	)
	if err != nil || attempts != 3 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
	if len(retries) != 2 || retries[0] != 1 || retries[1] != 2 {
		t.Fatalf("retry notices = %v", retries)
	}
}

func TestRetryProviderRequestStopsOnPermanentFailure(t *testing.T) {
	attempts := 0
	err := retryProviderRequest(
		retryRequestTestContext(2, nil),
		func() error {
			attempts++
			return errors.New("release is missing dlc.dat")
		},
	)
	if err == nil || attempts != 1 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestRetryProviderRequestReportsExhaustedAttempts(t *testing.T) {
	attempts := 0
	err := retryProviderRequest(
		retryRequestTestContext(2, nil),
		func() error {
			attempts++
			return markProviderErrorRetryable(errors.New("temporary network failure"))
		},
	)
	if err == nil || attempts != 3 || err.Error() != "after 3 attempts: temporary network failure" {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestRetryProviderRequestHonorsCancellation(t *testing.T) {
	baseCtx, cancel := context.WithCancel(context.Background())
	ctx := context.WithValue(baseCtx, providerRetryContextKey{}, providerRetryConfig{
		retries:    2,
		retryDelay: time.Millisecond,
		onRetry: func(_, _ int, _ time.Duration, _ error) {
			cancel()
		},
	})
	attempts := 0
	err := retryProviderRequest(ctx, func() error {
		attempts++
		return markProviderErrorRetryable(errors.New("temporary network failure"))
	})
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}
