package roblox

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Maxxxxxx-x/rotector-slop/models"
	"github.com/Maxxxxxx-x/rotector-slop/utils"
)

type User struct {
	Id   int    `json:"userId"`
	Name string `json:"username"`
}

type Role struct {
	Name string `json:"name"`
}

type GroupUserRecord struct {
	User User `json:"user"`
	Role Role `json:"role"`
}

type GroupUsersResponse struct {
	NextPageCursor *string           `json:"nextPageCursor"`
	Data           []GroupUserRecord `json:"data"`
}

type GroupUserChunk struct {
	GroupId string
	Users   map[string]models.User
}

const GROUP_USER_API = "https://groups.roblox.com/v1/groups/%s/users?sortOrder=Asc&limit=100&cursor=%s"

func GetMembersFromGroup(cookie string, groupId string, chunkChan chan<- GroupUserChunk) error {
	cursor := ""

	for {
		groupLimiter.Wait()
		api := fmt.Sprintf(GROUP_USER_API, groupId, cursor)

		var resp *http.Response
		var err error
		maxRetries := 5

		for attempt := range maxRetries {
			req, err := http.NewRequest(http.MethodGet, api, nil)
			if err != nil {
				return fmt.Errorf("Failed to build reuqest obj: %w", err)
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
				return fmt.Errorf("Bad request (400) for group %s: group is invali / DNE, or malformed cursor", groupId)
			}

			if resp.StatusCode == http.StatusForbidden {
				resp.Body.Close()
				return fmt.Errorf("Forbidden (403) for group %s: No permissions to get member list", groupId)
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
					return fmt.Errorf("Roblox group API returned %d", resp.StatusCode)
				}
				time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
				continue
			}
			break
		}

		var rbxResp GroupUsersResponse
		err = json.NewDecoder(resp.Body).Decode(&rbxResp)
		resp.Body.Close()
		if err != nil {
			return err
		}

		pageMap := make(map[string]models.User, len(rbxResp.Data))
		for _, record := range rbxResp.Data {
			userId := strconv.Itoa(record.User.Id)
			pageMap[userId] = models.User{
				Name: record.User.Name,
				Role: record.Role.Name,
			}
		}

		if len(pageMap) > 0 {
			chunkChan <- GroupUserChunk{
				GroupId: groupId,
				Users:   pageMap,
			}
		}

		if rbxResp.NextPageCursor == nil || *rbxResp.NextPageCursor == "" || *rbxResp.NextPageCursor == "null" {
			break
		}
		cursor = *rbxResp.NextPageCursor
	}

	return nil
}

func StreamUsersFromGroups(cookie string, groups map[string]string, chunkChan chan<- GroupUserChunk, errChan chan<- error) {
	if len(groups) == 0 {
		close(chunkChan)
		close(errChan)
		return
	}

	groupIds := make([]string, 0, len(groups))
	for id := range groups {
		groupIds = append(groupIds, id)
	}

	midPoint := (len(groupIds) + 1) / 2
	var chunks [][]string
	if len(groupIds) == 1 {
		chunks = [][]string{groupIds}
	} else {
		chunks = [][]string{
			groupIds[:midPoint],
			groupIds[midPoint:],
		}
	}
	var wg sync.WaitGroup

	for _, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}

		targetGroupIds := chunk
		wg.Go(func() {
			for _, id := range targetGroupIds {
				if err := GetMembersFromGroup(cookie, id, chunkChan); err != nil {
					errChan <- fmt.Errorf("Error pulling records from %s(%s): %w", groups[id], id, err)
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
