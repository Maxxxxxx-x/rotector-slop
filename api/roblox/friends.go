package roblox

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Maxxxxxx-x/rotector-slop/models"
	"github.com/Maxxxxxx-x/rotector-slop/utils"
)

type FriendRecord struct {
	Id               int  `json:"id"`
	HasVerifiedBadge bool `json:"hasVerifiedBadge"`
}

type FriendPageResponse struct {
	PreviousCursor string         `json:"PreviousCursor"`
	PageItems      []FriendRecord `json:"PageItems"`
	NextCursor     string         `json:"NextCursor"`
}

type FriendChunk struct {
	TargetId  string
	FriendIds []string
}

type MetadataResponse struct {
	Username string `json:"userName"`
}

type Metadata struct {
	UserId   string
	Username string
}

const (
	PAGINATED_FRIEND_LIST_API = "https://friends.roblox.com/v1/users/%s/friends/find?userSort=2&limit=50&cursor=%s"
	ACCOUNT_METADATA_API      = "https://friends.roblox.com/v1/metadata?targetUserId=%s"
)

var (
	metadataCache = models.NewCache[string, string]()
	friendsCache  = models.NewCache[string, bool]()
)

type HTTPStatus struct {
	Code int
	Text string
}

func doMetadataRequest(cookie string, api string, userId string, attempt int, maxRetries int) (*http.Response, time.Duration, HTTPStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return nil, 0, HTTPStatus{}, fmt.Errorf("failed to build metadata request: %w", err)
	}

	if cookie != "" {
		req.Header.Set("Cookie", fmt.Sprintf(".ROBLOSECURITY=%s", cookie))
	}
	resp, err := rbxClient.Do(req)
	if err != nil {
		if attempt == maxRetries-1 {
			return nil, 0, HTTPStatus{}, fmt.Errorf("network error on metadata after max retries: %w", err)
		}
		return nil, time.Duration(math.Pow(2, float64(attempt))) * time.Second, HTTPStatus{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		status := HTTPStatus{
			Code: resp.StatusCode,
			Text: resp.Status,
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfterHeader := resp.Header.Get("Retry-After")
			resp.Body.Close()
			return nil, utils.CalculateRetryDelay(retryAfterHeader, attempt), status, nil
		}
		resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusBadRequest:
			return nil, 0, status, fmt.Errorf("Bad request for metadata on user: %s", userId)
		case http.StatusForbidden:
			return nil, 0, status, fmt.Errorf("Forbidden for metadata on user: %s", userId)
		default:
			if attempt == maxRetries-1 {
				return nil, 0, status, fmt.Errorf("Metadata API returned status %d for user %s with status: (%d) %s", resp.StatusCode, userId, resp.StatusCode, resp.Status)
			}
			return nil, time.Duration(math.Pow(2, float64(attempt))) * time.Second, status, nil

		}
	}

	return resp, 0, HTTPStatus{}, nil
}

func GetMetadataOfUser(cookie string, userId string) (Metadata, error) {
	if cachedUsername, exists := metadataCache.Get(userId); exists {
		return Metadata{
			UserId:   userId,
			Username: cachedUsername,
		}, nil
	}

	friendLimiter.Wait()
	api := fmt.Sprintf(ACCOUNT_METADATA_API, userId)

	maxRetries := 5
	attempt := 0
	var lastStatus HTTPStatus

	for attempt < maxRetries {
		resp, backoffDur, status, err := doMetadataRequest(cookie, api, userId, attempt, maxRetries)
		if err != nil {
			return Metadata{}, err
		}

		if status.Code != 0 {
			lastStatus = status
		}

		if backoffDur > 0 {
			log.Printf("[RETRYING] Status %d for %s. Sleeping %v\n", status.Code, userId, backoffDur)
			time.Sleep(backoffDur)
			if attempt == maxRetries-1 {
				attempt = 0
			} else {
				attempt += 1
			}

			continue
		}
		var rbxResp MetadataResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&rbxResp)
		resp.Body.Close()

		if decodeErr != nil {
			return Metadata{}, fmt.Errorf("Failed to parse metadata payload: %w", err)
		}

		metadataCache.Set(userId, rbxResp.Username)
		return Metadata{
			UserId:   userId,
			Username: rbxResp.Username,
		}, nil
	}

	return Metadata{}, fmt.Errorf("Failed to fetch metadata for user %s after %d attempts. Status: %d (%v)", userId, maxRetries, lastStatus.Code, lastStatus.Text)
}

func GetFriendsFromUser(cookie string, userId string, chunkChan chan<- FriendChunk) error {
	if _, isCached := friendsCache.Get(userId); isCached {
		return nil
	}

	cursor := ""
	ctx := context.Background()

	for {
		friendLimiter.Wait()
		api := fmt.Sprintf(PAGINATED_FRIEND_LIST_API, userId, cursor)

		var resp *http.Response
		var err error
		maxRetries := 5

		for attempt := range maxRetries {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
			if err != nil {
				return fmt.Errorf("Failed to build request obj: %w", err)
			}

			if cookie != "" {
				req.Header.Set("Cookie", fmt.Sprintf(".ROBLOSECURITY=%s", cookie))
			}

			resp, err = rbxClient.Do(req)
			if err != nil {
				if attempt == maxRetries-1 {
					return fmt.Errorf("Network error after max retries: %v", err)
				}
				time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
				continue
			}

			if resp.StatusCode == http.StatusBadRequest {
				resp.Body.Close()
				return fmt.Errorf("bad request for user %s: Invalid id / cursor or bad params", userId)
			}

			if resp.StatusCode == http.StatusForbidden {
				resp.Body.Close()
				return fmt.Errorf("forbidden for user %s: profile visibility or permission restricted", userId)
			}

			if resp.StatusCode == http.StatusTooManyRequests {
				retryAfterHeader := resp.Header.Get("Retry-After")
				resp.Body.Close()
				backoffDuration := utils.CalculateRetryDelay(retryAfterHeader, attempt)
				time.Sleep(backoffDuration)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				if attempt == maxRetries-1 {
					return fmt.Errorf("friend API returned status %d", resp.StatusCode)
				}
				time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
				continue
			}
			break
		}

		var rbxResp FriendPageResponse
		err = json.NewDecoder(resp.Body).Decode(&rbxResp)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("Faield to decode friend page: %w", err)
		}

		friendIds := make([]string, 0, len(rbxResp.PageItems))
		for _, record := range rbxResp.PageItems {
			if record.Id == -1 {
				continue
			}
			friendIds = append(friendIds, strconv.Itoa(record.Id))
		}

		if len(friendIds) > 0 {
			chunkChan <- FriendChunk{
				TargetId:  userId,
				FriendIds: friendIds,
			}
		}

		if rbxResp.NextCursor == "" || rbxResp.NextCursor == "null" {
			break
		}
		cursor = rbxResp.NextCursor
	}

	friendsCache.Set(userId, true)
	return nil
}

func StreamFriendsFromUser(cookie string, userIds []string, chunkChan chan<- FriendChunk, errChan chan<- error) {
	if len(userIds) == 0 {
		close(chunkChan)
		close(errChan)
		return
	}

	midPoint := (len(userIds) + 1) / 2
	var chunks [][]string

	if len(userIds) == 1 {
		chunks = [][]string{userIds}
	} else {
		chunks = [][]string{
			userIds[:midPoint],
			userIds[midPoint:],
		}
	}

	var wg sync.WaitGroup

	for _, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}

		targetIds := chunk
		wg.Go(func() {
			for _, id := range targetIds {
				if err := GetFriendsFromUser(cookie, id, chunkChan); err != nil {
					errChan <- fmt.Errorf("error streaming friends for user %s: %w", id, err)
				}
			}
		})
	}

	go func() {
		wg.Wait()
		close(chunkChan)
		close(errChan)
	}()
}
