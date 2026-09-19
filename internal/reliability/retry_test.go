package reliability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryEventuallySucceeds(t *testing.T) {
	t.Parallel()
	attempts := 0
	result, retries, err := Retry(context.Background(), RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}, func(context.Context) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("temporary")
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok" || retries != 2 {
		t.Fatalf("result = %q retries = %d", result, retries)
	}
}

func TestRetryHonorsCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := Retry(ctx, RetryPolicy{MaxAttempts: 3, BaseDelay: time.Second}, func(context.Context) (string, error) {
		return "", errors.New("temporary")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v; want context canceled", err)
	}
}
