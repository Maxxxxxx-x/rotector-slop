package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"maps"
	"sync"

	"github.com/Maxxxxxx-x/rotector-slop/api/roblox"
	"github.com/Maxxxxxx-x/rotector-slop/api/rotector"
	"github.com/Maxxxxxx-x/rotector-slop/config"
	"github.com/Maxxxxxx-x/rotector-slop/db"
	"github.com/Maxxxxxx-x/rotector-slop/db/sqlc"
	"github.com/Maxxxxxx-x/rotector-slop/models"
	"github.com/Maxxxxxx-x/rotector-slop/utils"
)

const TEST_USER_ID = "502726319"

var dbMu sync.Mutex

func main() {
	cfg, err := config.GetFromEnv(".env")
	if err != nil {
		log.Fatal(err)
		return
	}

	db, err := db.ConnectDB(cfg.Database)
	if err != nil {
		log.Fatal(err)
		return
	}

	for groupId, groupName := range cfg.Groups {
		if _, err := db.CreateGroup(context.Background(), sqlc.CreateGroupParams{
			ID:      groupId,
			Name:    groupName,
			Members: 0,
		}); err != nil {
			log.Printf("Failed to store %s(%s) to database: %v\n", groupName, groupId, err)
		}
	}

	groupMemberList, err := fetchUsersFromGroups(cfg.RobloxCookie, cfg.Groups)
	if err != nil {
		log.Fatalf("Failed to get users from groups: %v", err)
	}

	categorized, uniqueUsers, err := utils.PartiitionGroups(groupMemberList)
	if err != nil {
		log.Fatalf("Failed to categorize users: %v", err)
	}

	var userIds []string
	fmt.Printf("Unqiue users: %d\n", len(uniqueUsers))
	for groupId, ids := range categorized {
		fmt.Printf("%s(%s): %d users\n", cfg.Groups[groupId], groupId, len(ids))
		userIds = append(userIds, ids...)
	}

	var mu sync.RWMutex
	var wg sync.WaitGroup

	for groupId, users := range categorized {
		for _, userId := range users {
			user := uniqueUsers[userId]
			groupRole := sql.NullString{
				String: user.Role,
				Valid:  true,
			}

			name := sql.NullString{
				String: user.Name,
				Valid:  true,
			}
			_, err := db.GetUserById(context.Background(), userId)
			if err == nil {
				if err := db.FullUpdateUser(context.Background(), sqlc.FullUpdateUserParams{
					Name:    name,
					Role:    groupRole,
					Groupid: groupId,
				}); err != nil {
					log.Printf("Failed to update user %s: %v\n", userId, err)
				}
				log.Printf("[UPDATE] Updated user record for %s\n", userId)
				continue
			} else if err == sql.ErrNoRows {
				if _, err := db.CreateUser(context.Background(), sqlc.CreateUserParams{
					ID:      userId,
					Name:    name,
					Role:    groupRole,
					Groupid: groupId,
				}); err != nil {
					log.Printf("[CREATE] Failed to create user record for %s: %v\n", userId, err)
				}
				log.Printf("[CREATE] Created user record for %s\n", userId)
				continue
			} else {
				log.Printf("Error checking user %s: %v\n", userId, err)
			}

		}
	}

	wg.Go(func() {
		err := fetchRotectorRecords(db, &mu, cfg.RotectorKey, &uniqueUsers, userIds)
		if err != nil {
			fmt.Printf("Failed to get flags: %v\n", err)
		}
	})

	wg.Go(func() {
		err := fetchFriendsFromUsers(db, &mu, cfg.RobloxCookie, &uniqueUsers, userIds)
		if err != nil {
			fmt.Printf("Failed to fetch friends: %v\n", err)
		}
	})

	wg.Wait()

	for id, user := range uniqueUsers {
		fmt.Printf("%s(%s) | Friends: %d\n | Flagged: %s\n", user.Name, id, len(user.Friends), user.Flags.FlagType)
	}

	var usersToQuery []string
	newUsersToQuery := make(map[string]models.User)

	for userId, user := range uniqueUsers {
		if user.Role != "" {
			continue
		}
		usersToQuery = append(usersToQuery, userId)
		newUsersToQuery[userId] = user
	}

	err = fetchRotectorRecords(db, &mu, cfg.RotectorKey, &newUsersToQuery, usersToQuery)
	if err != nil {
		fmt.Printf("Failed to fetch records: %v\n", err)
	}

	maps.Copy(uniqueUsers, newUsersToQuery)

	fmt.Printf("Total users: %d\n", len(uniqueUsers))
}

func fetchUsersFromGroups(cookie string, groups map[string]string) (map[string]map[string]models.User, error) {
	final := make(map[string]map[string]models.User)
	if len(groups) == 0 {
		return final, nil
	}

	for id := range groups {
		final[id] = make(map[string]models.User)
	}

	chunkChan := make(chan roblox.GroupUserChunk, 50)
	errChan := make(chan error, len(groups))

	roblox.StreamUsersFromGroups(cookie, groups, chunkChan, errChan)

	go func() {
		for err := range errChan {
			log.Printf("[GROUP CHUNK] %v\n", err)
		}
	}()

	for chunk := range chunkChan {
		maps.Copy(final[chunk.GroupId], chunk.Users)
		fmt.Printf("[GROUP CHUNK] Fetched %d users from %s(%s)\n", len(final[chunk.GroupId]), groups[chunk.GroupId], chunk.GroupId)
	}

	return final, nil
}

func fetchFriendsFromUsers(db *db.DB, mu *sync.RWMutex, robloxCookie string, users *map[string]models.User, userIds []string) error {
	if users == nil || *users == nil {
		return fmt.Errorf("User ptr is nil")
	}

	if len(userIds) == 0 {
		return nil
	}

	friendsChan := make(chan roblox.FriendChunk, 100)
	friendsErrChan := make(chan error, 100)

	roblox.StreamFriendsFromUser(robloxCookie, userIds, friendsChan, friendsErrChan)

	var wg sync.WaitGroup

	go func() {
		for err := range friendsErrChan {
			log.Printf("[FRIEND LIST ERROR] %v\n", err)
		}
	}()

	for chunk := range friendsChan {
		fmt.Printf("Fetched %d friends for %s\n", len(chunk.FriendIds), chunk.TargetId)

		mu.Lock()
		if user, exist := (*users)[chunk.TargetId]; exist {
			user.Friends = append(user.Friends, chunk.FriendIds...)
			(*users)[chunk.TargetId] = user
		}
		mu.Unlock()

		currentFriendIds := chunk.FriendIds
		currentId := chunk.TargetId
		wg.Go(func() {
			for _, userId := range currentFriendIds {
				dbMu.Lock()
				_, err := db.GetUserById(context.Background(), userId)
				if err != nil {
					if err == sql.ErrNoRows {
						if _, err := db.CreateUser(context.Background(), sqlc.CreateUserParams{
							ID: userId,
						}); err != nil {
							log.Printf("[CHUNK] Failed to create user %s: %v\n", userId, err)
						}
					} else {
						log.Printf("[CHUNK] Erorr checking user %s: %v\n", userId, err)
					}
				}

				if _, err := db.CreateConnection(context.Background(), sqlc.CreateConnectionParams{
					ID:     fmt.Sprintf("%s_%s", currentId, userId),
					Source: currentId,
					Target: userId,
				}); err != nil {
					log.Printf("Failed to create connection for %s: %v\n", currentId, err)
				}
				dbMu.Unlock()
			}
		})

		wg.Go(func() {
			for _, userId := range currentFriendIds {
				metadata, err := roblox.GetMetadataOfUser(robloxCookie, userId)
				if err != nil {
					log.Printf("[METADATA] Failed to get metdata of user %s: %v", userId, err)
					continue
				}
				fmt.Printf("Fetched metadata for %s\n", userId)
				mu.Lock()
				user := (*users)[metadata.UserId]
				user.Name = metadata.Username
				(*users)[metadata.UserId] = user
				mu.Unlock()

				userName := sql.NullString{
					String: metadata.Username,
					Valid:  true,
				}

				dbMu.Lock()
				if err := db.UpdateUserName(context.Background(), sqlc.UpdateUserNameParams{
					ID:   metadata.UserId,
					Name: userName,
				}); err != nil {
					log.Printf("Failed to save metadata for %s: %v\n", metadata.UserId, err)
				}
				dbMu.Unlock()
			}
		})
	}

	wg.Wait()

	return nil
}

func fetchRotectorRecords(db *db.DB, mu *sync.RWMutex, apiKey string, users *map[string]models.User, userIds []string) error {
	if users == nil || *users == nil {
		return fmt.Errorf("user ptr is nil")
	}

	if len(userIds) == 0 {
		return nil
	}

	chunkChan := make(chan rotector.RotectorChunk, len(*users))
	errChan := make(chan error, 100)

	rotector.StreamRotectorRecords(apiKey, userIds, chunkChan, errChan)

	go func() {
		for err := range errChan {
			log.Printf("[ROTECTOR ERROR] %v\n", err)
		}
	}()

	for chunk := range chunkChan {
		category := sql.NullString{
			String: chunk.Record.Category,
			Valid:  true,
		}

		confidence := sql.NullFloat64{
			Float64: chunk.Record.Confidence,
			Valid:   true,
		}

		isReportable := sql.NullInt64{
			Int64: utils.BoolToInt64(chunk.Record.IsReportable),
			Valid: true,
		}

		isLocked := sql.NullInt64{
			Int64: utils.BoolToInt64(chunk.Record.IsLocked),
			Valid: true,
		}

		reviewer := sql.NullString{
			String: chunk.Record.Reviewer,
			Valid:  true,
		}

		queuedAt := sql.NullString{
			String: chunk.Record.QueuedAt,
			Valid:  true,
		}

		processedAt := sql.NullString{
			String: chunk.Record.ProcessedAt,
			Valid:  true,
		}

		lastUpdated := sql.NullString{
			String: chunk.Record.LastUpdated,
			Valid:  true,
		}

		reasons := sql.NullString{
			String: utils.ToJson(chunk.Record.Reasons),
			Valid:  true,
		}

		dbMu.Lock()
		_, err := db.CreateFlag(context.Background(), sqlc.CreateFlagParams{
			ID:           chunk.UserId,
			FlagType:     chunk.Record.FlagType,
			Category:     category,
			Confidence:   confidence,
			Reviewer:     reviewer,
			IsReportable: isReportable,
			IsLocked:     isLocked,
			QueuedAt:     queuedAt,
			ProcessedAt:  processedAt,
			LastUpdated:  lastUpdated,
			Reasons:      reasons,
		})
		dbMu.Unlock()
		if err != nil {
			return err
		}

		mu.Lock()
		if user, exists := (*users)[chunk.UserId]; exists {
			user.Flags = chunk.Record
			(*users)[chunk.UserId] = user
		}

		mu.Unlock()
		fmt.Printf("[ROTECTOR CHUNK] Fetched flags for user %s\n", chunk.UserId)
	}

	return nil
}
