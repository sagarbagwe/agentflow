package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
	key    string
}

func NewRedis(client *redis.Client, key string) *Redis {
	return &Redis{client: client, key: key}
}

func (q *Redis) Enqueue(ctx context.Context, job Job) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode queue job: %w", err)
	}
	if err := q.client.LPush(ctx, q.key, payload).Err(); err != nil {
		return fmt.Errorf("enqueue job: %w", err)
	}
	return nil
}

func (q *Redis) Dequeue(ctx context.Context) (Job, error) {
	values, err := q.client.BRPop(ctx, 0, q.key).Result()
	if err != nil {
		return Job{}, fmt.Errorf("dequeue job: %w", err)
	}
	if len(values) != 2 {
		return Job{}, fmt.Errorf("unexpected Redis queue response")
	}
	var job Job
	if err := json.Unmarshal([]byte(values[1]), &job); err != nil {
		return Job{}, fmt.Errorf("decode queue job: %w", err)
	}
	return job, nil
}
