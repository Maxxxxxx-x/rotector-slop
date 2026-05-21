package analyzer

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/Maxxxxxx-x/rotector-slop/api"
	"github.com/Maxxxxxx-x/rotector-slop/models"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

type RateLimitError struct {
	ResetSeconds int
	Underlying   error
}

func (err *RateLimitError) Error() string {
	return fmt.Errorf("Rate limited: %w (resetting in %ds)", err.Underlying, err.ResetSeconds).Error()
}

func BuildConnections(targets map[string]string, concurrencyLimit int) (map[string]models.Friend, error) {
	if len(targets) == 0 {
		return make(map[string]models.Friend), nil
	}

	return compileFriendGraph(targets, concurrencyLimit)
}

func isPrivacyOrMissing(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "403") || strings.Contains(msg, "404")
}

func compileFriendGraph(targets map[string]string, concurrencyLimit int) (map[string]models.Friend, error) {
	graph := make(map[string]models.Friend)

	type target struct {
		id   string
		name string
	}

	type jobResult struct {
		targetId string
		friends  []string
	}

	jobs := make(chan target, len(targets))
	results := make(chan jobResult, len(targets))

	group, ctx := errgroup.WithContext(context.Background())
	limiter := rate.NewLimiter(rate.Limit(1.5), 2)

	var mutex sync.Mutex

	for id, name := range targets {
		jobs <- target{
			id:   id,
			name: name,
		}
	}
	close(jobs)

	for range concurrencyLimit {
		group.Go(func() error {
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			for target := range jobs {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				mutex.Lock()
				currentLimiter := limiter
				mutex.Unlock()

				if err := currentLimiter.Wait(ctx); err != nil {
					return err
				}

				maxAttempts := 5

				for attempt := 1; attempt <= maxAttempts; attempt++ {
					friendsMap, err := api.GetFriendsOfUser(target.id, "")
					if err == nil {
						friendIds := make([]string, 0, len(friendsMap))
						for friendId := range friendsMap {
							if _, isOriginalUser := targets[friendId]; isOriginalUser {
								continue
							}
							friendIds = append(friendIds, friendId)
						}

						fmt.Printf("[BUILD CONNECTIONS] [Attempt %d/%d] Fetching friends for %s(%s): Fetched %d friends\n", attempt, maxAttempts, target.name, target.id, len(friendsMap))

						results <- jobResult{
							targetId: target.id,
							friends:  friendIds,
						}
						break
					}
					if isPrivacyOrMissing(err) {
						fmt.Printf("[BUILD CONNECTIONS] [Attempt %d/%d] Fetching friends for %s(%s): Skipped due to privacy settings or missing account\n", attempt, maxAttempts, target.name, target.id)
						break
					}

					if strings.Contains(err.Error(), "429") && attempt < maxAttempts {
						var waitDuration time.Duration
						var rateLimitErr *api.RateLimitError

						if errors.As(err, &rateLimitErr) && rateLimitErr.ResetSeconds > 0 {
							jitter := time.Duration(500+rng.Intn(2000)) * time.Millisecond
							waitDuration = (time.Duration(rateLimitErr.ResetSeconds+2) * time.Second) + jitter
						} else {
							waitDuration = time.Duration(60+rng.Intn(5)) * time.Second
						}

						fmt.Printf("[BUILD CONNECTIONS] [Attempt %d/%d] Fetching friends for %s(%s): Rate limit reached. Retrying in %v\n", attempt, maxAttempts, target.name, target.id, waitDuration)
						mutex.Lock()
						limiter.SetLimit(rate.Limit(0.2))
						mutex.Unlock()

						select {
						case <-time.After(waitDuration):
							mutex.Lock()
							limiter.SetLimit(rate.Limit(1.5))
							mutex.Unlock()
							continue
						case <-ctx.Done():
							return ctx.Err()
						}
					}

					if attempt < maxAttempts {
						backoff := time.Duration(1<<attempt)*time.Second + (time.Duration(rng.Intn(1000)) * time.Millisecond)
						fmt.Printf("[BUILD CONNECTIONS] [Attempt %d/%d] Fetching friends for %s(%s): Unexpected Error (%v). Retrying in %v...\n", attempt, maxAttempts, target.name, target.id, err.Error(), backoff)

						select {
						case <-time.After(backoff):
							continue
						case <-ctx.Done():
							return ctx.Err()
						}
					}

					break
				}
			}

			return nil
		})
	}

	go func() {
		_ = group.Wait()
		close(results)
	}()

	if err := group.Wait(); err != nil {
		return nil, err
	}

	for res := range results {
		for _, friendId := range res.friends {
			item, exists := graph[friendId]
			if !exists {
				item = models.Friend{
					FriendsWith: make([]string, 0, 4),
				}
			}
			item.FriendsWith = append(item.FriendsWith, res.targetId)
			graph[friendId] = item
		}
	}

	return graph, nil
}
