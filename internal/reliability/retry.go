package reliability

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"
)

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

func Retry[T any](ctx context.Context, policy RetryPolicy, operation func(context.Context) (T, error)) (T, int, error) {
	var zero T
	if policy.MaxAttempts < 1 {
		return zero, 0, fmt.Errorf("max attempts must be positive")
	}

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		result, err := operation(ctx)
		if err == nil {
			return result, attempt - 1, nil
		}
		if attempt == policy.MaxAttempts {
			return zero, attempt - 1, err
		}

		delay := policy.BaseDelay << (attempt - 1)
		if policy.MaxDelay > 0 && delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
		jitter := time.Duration(rand.Int64N(max(int64(delay/2), 1)))
		timer := time.NewTimer(delay + jitter)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, attempt - 1, ctx.Err()
		case <-timer.C:
		}
	}
	return zero, policy.MaxAttempts - 1, fmt.Errorf("retry exhausted")
}
