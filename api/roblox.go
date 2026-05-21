package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Maxxxxxx-x/rotector-slop/models"
)

type RateLimitError struct {
	StatusCode   int
	ResetSeconds int
	Err          error
}

func (err *RateLimitError) Error() string {
	return fmt.Sprintf("%v (status %d, reset window: %ds)", err.Err, err.StatusCode, err.ResetSeconds)
}

type RobloxUser struct {
	UserId   int    `json:"userId"`
	Username string `json:"username"`
}

type RobloxRole struct {
	Name string `json:"name"`
}

type RobloxGroupRecord struct {
	User RobloxUser `json:"user"`
	Role RobloxRole `json:"role"`
}

type RobloxGroupResponse struct {
	NextPageCursor *string             `json:"nextPageCursor"`
	Data           []RobloxGroupRecord `json:"data"`
}

type RobloxFriendItem struct {
	Id int `json:"id"`
}

type RobloxFriendResponse struct {
	NextCursor *string            `json:"NextCursor"`
	PageItems  []RobloxFriendItem `json:"PageItems"`
}

var rbxClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

func GetMembersFromGroup(groupId string, startingCursor string) (map[string]models.User, error) {
	userMap := make(map[string]models.User, 100)
	cursor := startingCursor

	var resp *http.Response
	var err error

	for {
		api := fmt.Sprintf("https://groups.roblox.com/v1/groups/%s/users?sortOrder=Asc&limit=100&cursor=%s", groupId, cursor)

		resp, err = rbxClient.Get(api)
		if err != nil {
			return userMap, err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return userMap, fmt.Errorf("Roblox group api returned status %d", resp.StatusCode)
		}

		var rbxResp RobloxGroupResponse
		err = json.NewDecoder(resp.Body).Decode(&rbxResp)
		resp.Body.Close()
		if err != nil {
			return userMap, err
		}

		for _, record := range rbxResp.Data {
			userId := strconv.Itoa(record.User.UserId)
			userMap[userId] = models.User{
				UserName: record.User.Username,
				RoleName: record.Role.Name,
			}
		}

		if rbxResp.NextPageCursor == nil || *rbxResp.NextPageCursor == "" || *rbxResp.NextPageCursor == "null" {
			break
		}
		cursor = *rbxResp.NextPageCursor
	}

	return userMap, nil
}

func GetFriendsOfUser(userId string, startingCursor string) (map[string]models.Friend, error) {
	friendMap := make(map[string]models.Friend, 50)
	cursor := startingCursor

	for {
		api := fmt.Sprintf("https://friends.roblox.com/v1/users/%s/friends/find?limit=50&cursor=%s", userId, cursor)

		resp, err := rbxClient.Get(api)
		if err != nil {
			return friendMap, err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resetHeader := resp.Header.Get("x-ratelimit-reset")
			resp.Body.Close()

			resetSec, convertErr := strconv.Atoi(resetHeader)
			if convertErr != nil {
				resetSec = 0
			}

			return friendMap, &RateLimitError{
				StatusCode:   http.StatusTooManyRequests,
				ResetSeconds: resetSec,
				Err:          errors.New("Roblox friend API returned 429"),
			}
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return friendMap, fmt.Errorf("Roblox friend api returned status %d", resp.StatusCode)
		}

		var rbxResp RobloxFriendResponse
		err = json.NewDecoder(resp.Body).Decode(&rbxResp)
		resp.Body.Close()
		if err != nil {
			return friendMap, err
		}

		for _, friend := range rbxResp.PageItems {
			if friend.Id == -1 {
				continue
			}
			friendId := strconv.Itoa(friend.Id)
			friendMap[friendId] = models.Friend{}
		}

		if rbxResp.NextCursor == nil || *rbxResp.NextCursor == "" || *rbxResp.NextCursor == "null" {
			break
		}
		cursor = *rbxResp.NextCursor
	}

	return friendMap, nil
}
