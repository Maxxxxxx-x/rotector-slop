package api

import (
	"fmt"
	"sync"
	"time"
)

type RateLimitError struct {
	StatusCode   int
	ResetSeconds int
	Err          error
}

func (err *RateLimitError) Error() string {
	return fmt.Sprintf("%v (status: %d, reset window: %ds)", err.Err, err.StatusCode, err.ResetSeconds)
}

type Bucket struct {
	rate         time.Duration
	maxTokens    float64
	tokens       float64
	lastRefilled time.Time
	mu           sync.Mutex
}

func NewBucket(rate time.Duration, maxTokens float64) *Bucket {
	return &Bucket{
		rate:         rate,
		maxTokens:    maxTokens,
		tokens:       maxTokens,
		lastRefilled: time.Now(),
	}
}

func (bucket *Bucket) Wait() {
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	for {
		now := time.Now()
		elapsed := now.Sub(bucket.lastRefilled)
		bucket.lastRefilled = now

		bucket.tokens += float64(elapsed) / float64(bucket.rate)
		if bucket.tokens > bucket.maxTokens {
			bucket.tokens = bucket.maxTokens
		}

		if bucket.tokens >= 1.0 {
			bucket.tokens -= 1.0
			return
		}

		sleepTime := time.Duration((1.0 - bucket.tokens) * float64(bucket.rate))
		bucket.mu.Unlock()
		time.Sleep(sleepTime)
		bucket.mu.Lock()
	}
}
