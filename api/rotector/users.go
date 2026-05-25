package rotector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Maxxxxxx-x/rotector-slop/api"
	"github.com/Maxxxxxx-x/rotector-slop/models"
	"github.com/Maxxxxxx-x/rotector-slop/utils"
)

const BASE_URL = "https://roscoe.rotector.com"

type Payload struct {
	Ids         []string `json:"ids"`
	ExcludeInfo bool     `json:"excludeInfo"`
}

type RotectorChunk struct {
	UserId string
	Record models.ProcessedFlagData
}

var (
	rotectorCache = models.NewCache[string, models.ProcessedFlagData]()
	rateLimiter   = api.NewBucket(200*time.Millisecond, 50)
)

func formatRFC3339(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

func mapFlagType(flagType int) string {
	switch flagType {
	case 0:
		return "Unflagged"
	case 1:
		return "Flagged"
	case 2:
		return "Confirmed"
	case 3:
		return "Queued"
	case 4:
		return "Provisional flag"
	case 5:
		return "Mixed"
	case 6:
		return "Past offender"
	case 8:
		return "Redacted"
	default:
		return fmt.Sprintf("Unknown (flagType: %d)", flagType)
	}
}

func mapCategory(category int) string {
	switch category {
	case 1:
		return "CSAM"
	case 2:
		return "Sexual"
	case 3:
		return "Kink"
	case 4:
		return "Raceplay"
	case 5:
		return "Condo"
	case 6:
		return "Other"
	default:
		return "None"
	}
}

func BatchFetchUserFlags(apiKey string, userIds []string, chunkChan chan<- RotectorChunk) error {
	if len(userIds) == 0 {
		return nil
	}

	ctx := context.Background()
	apiUrl := fmt.Sprintf("%s/v1/lookup/roblox/user", BASE_URL)

	body, err := json.Marshal(Payload{Ids: userIds, ExcludeInfo: false})
	if err != nil {
		return fmt.Errorf("Failed to marshal batch payload: %w", err)
	}

	var resp *http.Response
	maxRetries := 5

	for attempt := range maxRetries {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiUrl, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to build batch request obj: %w", err)
		}
		if apiKey != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err = rotectorClient.Do(req)
		if err != nil {
			if attempt == maxRetries-1 {
				return fmt.Errorf("network error during batch after max retries: %w", err)
			}
			time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
			continue
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
				return fmt.Errorf("rotector API returned status %d", resp.StatusCode)
			}
			time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
			continue
		}
		break
	}

	var batchRecord models.BatchFlagResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchRecord); err != nil {
		resp.Body.Close()
		return fmt.Errorf("failed to decode batch: %w", err)
	}
	resp.Body.Close()

	if !batchRecord.Success {
		return fmt.Errorf("Rotector API batch returned unsuccessful flag")
	}

	for userId, rawJson := range batchRecord.Data {
		var rawData models.FlagData
		if err := json.Unmarshal(rawJson, &rawData); err != nil {
			return fmt.Errorf("failed parsing raw records for user block %s: %w", userId, err)
		}
		processedRecord := models.ProcessedFlagData{
			Id:           rawData.Id,
			FlagType:     mapFlagType(rawData.FlagType),
			Category:     mapCategory(rawData.Category),
			Confidence:   rawData.Confidence,
			Reasons:      rawData.Reasons,
			Reviewer:     rawData.Reviewer.Username,
			IsReportable: rawData.IsReportable,
			IsLocked:     rawData.IsLocked,
			QueuedAt:     formatRFC3339(rawData.QueuedAt),
			ProcessedAt:  formatRFC3339(rawData.ProcessedAt),
			LastUpdated:  formatRFC3339(rawData.LastUpdated),
		}

		if processedRecord.Id <= 0 {
			if convertedId, parseErr := strconv.Atoi(userId); parseErr == nil {
				processedRecord.Id = convertedId
			}
		}
		rotectorCache.Set(userId, processedRecord)

		chunkChan <- RotectorChunk{
			UserId: userId,
			Record: processedRecord,
		}
	}

	return nil
}

func StreamRotectorRecords(apiKey string, userIds []string, chunkChan chan<- RotectorChunk, errChan chan<- error) {
	if len(userIds) == 0 {
		close(chunkChan)
		close(errChan)
		return
	}

	var toQuery []string

	for _, id := range userIds {
		if cachedRecord, exists := rotectorCache.Get(id); exists {
			chunkChan <- RotectorChunk{
				UserId: id,
				Record: cachedRecord,
			}
			continue
		}
		toQuery = append(toQuery, id)
	}

	if len(toQuery) == 0 {
		close(chunkChan)
		close(errChan)
		return
	}

	const batchSize = 100

	var batches [][]string
	for i := 0; i < len(toQuery); i += batchSize {
		end := min(i+batchSize, len(toQuery))
		batches = append(batches, toQuery[i:end])
	}

	midPoint := (len(batches) + 1) / 2
	var workerChunks [][][]string

	if len(batches) == 1 {
		workerChunks = [][][]string{batches}
	} else {
		workerChunks = [][][]string{
			batches[:midPoint],
			batches[midPoint:],
		}
	}

	var wg sync.WaitGroup

	for _, chunk := range workerChunks {
		if len(chunk) == 0 {
			continue
		}

		wg.Go(func() {
			for _, batch := range chunk {
				if err := BatchFetchUserFlags(apiKey, batch, chunkChan); err != nil {
					errChan <- fmt.Errorf("Error streaming batch lookup: %w", err)
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
