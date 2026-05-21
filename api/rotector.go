package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Maxxxxxx-x/rotector-slop/models"
)

const BASE_URL = "https://roscoe.rotector.com"

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
		return "FLagged"
	case 2:
		return "Confirmed"
	case 3:
		return "Queued"
	case 4:
		return "Provisional Flag"
	case 5:
		return "Mixed"
	case 6:
		return "Past Offender"
	case 8:
		return "Redacted"
	default:
		return fmt.Sprintf("Unknown (%d)", flagType)
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

var rotectorClient = &http.Client{
	Timeout: 15 * time.Second,
}

type Payload struct {
	Ids         []string `json:"ids"`
	ExcludeInfo bool     `json:"excludeInfo"`
}

func BatchGetRotectorUserFlags(userMap map[string]string) (map[string]models.ProcessedFlagData, error) {
	userIds := make([]string, 0, len(userMap))
	for key := range userMap {
		userIds = append(userIds, key)
	}

	finalResult := make(map[string]models.ProcessedFlagData)
	if len(userIds) == 0 {
		return finalResult, nil
	}

	var remaining = 50
	var resetTime time.Time
	var request *http.Request
	var response *http.Response
	var err error

	chunkSize := 100
	for i := 0; i < len(userIds); i += chunkSize {
		end := min(i+chunkSize, len(userIds))
		batchIds := userIds[i:end]

		if remaining <= 0 && time.Now().Before(resetTime) {
			sleepDuration := time.Until(resetTime)
			if sleepDuration > 0 {
				time.Sleep(sleepDuration)
			}
		}
		var payload []byte
		payload, err = json.Marshal(Payload{
			Ids:         batchIds,
			ExcludeInfo: false,
		})
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal payload: %w", err)
		}

		api := fmt.Sprintf("%s/v1/lookup/roblox/user", BASE_URL)
		request, err = http.NewRequest(http.MethodPost, api, bytes.NewBuffer(payload))
		if err != nil {
			return nil, fmt.Errorf("Failed to build request: %w", err)
		}
		request.Header.Add("Content-Type", "application/json")

		for {
			response, err = rotectorClient.Do(request)
			if err != nil {
				return nil, fmt.Errorf("Network error: %w", err)
			}

			if remainingHeader := response.Header.Get("X-RateLimit-Remaining"); remainingHeader != "" {
				if r, err := strconv.Atoi(remainingHeader); err == nil {
					remaining = r
				}
			}

			if resetHeader := response.Header.Get("X-RateLimit-Reset"); resetHeader != "" {
				if unixSecond, err := strconv.ParseInt(resetHeader, 10, 64); err == nil {
					resetTime = time.Unix(unixSecond, 0)
				}
			}

			if response.StatusCode == http.StatusTooManyRequests {
				response.Body.Close()

				retryAfter := 1
				if retryAfterHeader := response.Header.Get("Retry-After"); retryAfterHeader != "" {
					if sec, err := strconv.Atoi(retryAfterHeader); err == nil {
						retryAfter = sec
					}
				}
				time.Sleep(time.Duration(retryAfter) * time.Second)
				continue
			}
			break
		}

		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("Rotector API responded with status code %d", response.StatusCode)
		}

		var batchResponse models.BatchFlagResponse
		err = json.NewDecoder(response.Body).Decode(&batchResponse)
		response.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("Failed to decode body: %w", err)
		}

		if !batchResponse.Success {
			continue
		}

		for userId, rawElement := range batchResponse.Data {
			if string(rawElement) == "false" {
				continue
			}

			var flagData models.FlagData
			if err := json.Unmarshal(rawElement, &flagData); err != nil {
				return nil, fmt.Errorf("Failed to parse nested user info for %s: %w", userId, err)
			}

			if flagData.FlagType == 0 {
				continue
			}

			finalResult[userId] = models.ProcessedFlagData{
				Id:                   flagData.Id,
				FlagType:             mapFlagType(flagData.FlagType),
				Category:             mapCategory(flagData.Category),
				Confidence:           flagData.Confidence,
				Reasons:              flagData.Reasons,
				EngineVersion:        flagData.EngineVersion,
				VersionCompatibility: flagData.VersionCompatibility,
				IsReportable:         flagData.IsReportable,
				IsLocked:             flagData.IsLocked,
				QueuedAt:             formatRFC3339(flagData.QueuedAt),
				Processed:            flagData.Processed,
				ProcessedAt:          formatRFC3339(flagData.ProcessedAt),
				LastUpdated:          formatRFC3339(flagData.LastUpdated),
				MembershipBadge:      flagData.MembershipBadge,
			}
		}
	}

	return finalResult, nil
}
