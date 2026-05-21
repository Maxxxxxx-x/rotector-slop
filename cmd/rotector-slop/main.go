package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/Maxxxxxx-x/rotector-slop/api"
	"github.com/Maxxxxxx-x/rotector-slop/config"
	"github.com/Maxxxxxx-x/rotector-slop/models"
	"github.com/Maxxxxxx-x/rotector-slop/services/analyzer"
)

func main() {
	cfg, err := config.GetFromEnv(".env")
	if err != nil {
		log.Fatal(err)
		return
	}

	totalUsers := 0
	groupMembers := make(map[string]map[string]models.User)

	for id, name := range cfg.Groups {
		fmt.Printf("Fetching members from %s(%s)\n", id, name)

		userMap, err := api.GetMembersFromGroup(id, "")
		if err != nil {
			log.Fatal(err)
			return
		}

		groupMembers[id] = userMap

		count := len(userMap)
		totalUsers += count
		fmt.Printf("Fetched %d users from %s(%s)\n", count, name, id)
	}

	fmt.Printf("Total users: %d\n", totalUsers)
	fmt.Printf("Analyzing %d groups...\n", len(groupMembers))
	result := analyzer.AnalyzeGroups(groupMembers)
	fmt.Printf("Analyzed all groups\n")

	for groupId, members := range result.ExclusiveTo {
		fmt.Printf("Only in %s(%s): %d\n", cfg.Groups[groupId], groupId, len(members))
	}

	groupMembers = nil
	fmt.Printf("Group member size: %d\n", unsafe.Sizeof(groupMembers))
	runtime.GC()

	targets := make(map[string]string)

	for userId, user := range result.InAllGroups {
		targets[userId] = user.UserName
	}

	for _, group := range result.ExclusiveTo {
		for userId, user := range group {
			targets[userId] = user.UserName
		}
	}

	var wg sync.WaitGroup
	var buildConnectionsErr error
	var rotectorFlagsErr error

	var friendGraph map[string]models.Friend
	var flaggedUsers map[string]models.ProcessedFlagData

	concurrencyLimit := 8
	wg.Add(2)

	wg.Go(func() {
		fmt.Printf("[TASK] Started fetching all connections for %d users\n", totalUsers)
		friendGraph, buildConnectionsErr = analyzer.BuildConnections(targets, concurrencyLimit)
		if buildConnectionsErr != nil {
			fmt.Printf("[TASK ERROR] Connections build failed: %v\n", buildConnectionsErr)
			return
		}
		fmt.Println("[TASK] Finished fetching all connections")
	})

	wg.Go(func() {
		fmt.Printf("[TASK] Getting rotector flags for %d users\n", totalUsers)
		flaggedUsers, rotectorFlagsErr = api.BatchGetRotectorUserFlags(targets)
		if rotectorFlagsErr != nil {
			fmt.Printf("[TASK ERROR] Rotector lookup failed: %v\n", rotectorFlagsErr)
			return
		}
		fmt.Println("[TASK] Finished rotector lookup")
	})

	wg.Wait()

	if buildConnectionsErr != nil {
		log.Fatalf("BuildConnections error: %v\n", buildConnectionsErr)
	}
	if rotectorFlagsErr != nil {
		log.Fatalf("Rotector error: %v\n", rotectorFlagsErr)
	}

	fmt.Printf("[TASK] Scraping all %d connections\n", len(friendGraph))

	friendTargets := make(map[string]string)
	for friendId := range friendGraph {
		if _, alreadyChecked := targets[friendId]; !alreadyChecked {
			friendTargets[friendId] = ""
		}
	}

	var flaggedFriends map[string]models.ProcessedFlagData
	var rotectorFriendsErr error

	if len(friendTargets) > 0 {
		wg.Go(func() {
			fmt.Printf("[TASK] Getting rotector fllags for %d users\n", len(friendTargets))
			flaggedFriends, rotectorFriendsErr = api.BatchGetRotectorUserFlags(friendTargets)
			if rotectorFlagsErr != nil {
				fmt.Printf("[TASK ERROR] Connection Network lookup failed: %v\n", rotectorFriendsErr)
				return
			}
			fmt.Println("[TASK] Finished network lookup")
		})

		wg.Wait()
		if rotectorFriendsErr != nil {
			log.Fatalf("Network evaluationf ailed: %v\n", rotectorFriendsErr)
		}

		fmt.Println("Merging flag data")
		for friendId, flagData := range flaggedFriends {
			if friendObj, exists := friendGraph[friendId]; exists {
				friendObj.Flag = flagData
				friendGraph[friendId] = friendObj
			}
		}
	} else {
		fmt.Println("[TASK] No connections to scan")
	}

	fmt.Println("-----  ROTECTOR SLOP  -----")
	associationMap := make(map[string]models.AssociatedUser)
	flaggedFriendsReport := make(map[string]models.ProcessedFlagData)

	for friendId, friend := range friendGraph {
		if friend.Flag.FlagType != "" {
			flaggedFriendsReport[friendId] = friend.Flag

			for _, coreTgtId := range friend.FriendsWith {
				assoc, exists := associationMap[coreTgtId]
				if !exists {
					assoc = models.AssociatedUser{
						FlaggedFriends: make(map[string]string),
					}
				}
				assoc.FlaggedFriends[friendId] = friend.Flag.FlagType
				associationMap[coreTgtId] = assoc
			}
		}
	}

	writeReport := func(fileName string, users map[string]models.User) {
		report := models.GroupReport{
			Flagged:    make(map[string]models.ProcessedFlagData),
			Associated: make(map[string]models.AssociatedUser),
		}

		for userId := range users {
			if flagData, isFlagged := flaggedUsers[userId]; isFlagged {
				report.Flagged[userId] = flagData
			}
			if assocData, isAssociated := associationMap[userId]; isAssociated {
				report.Associated[userId] = assocData
			}
		}

		fileData, err := json.MarshalIndent(report, "", "    ")
		if err != nil {
			fmt.Printf("[ERROR] Failed processing json allocation %s: %v\n", fileName, err)
			return
		}

		if err := os.WriteFile(fileName, fileData, 0644); err != nil {
			fmt.Printf("[ERROR] Writer failure tracking towards %s: %v\n", fileName, err)
		} else {
			fmt.Printf("[FILE] Wrote slice to %s\n", fileName)
		}
	}

	writeReport("BothGroups.json", result.InAllGroups)
	for groupId := range cfg.Groups {
		writeReport(fmt.Sprintf("%s.json", groupId), result.ExclusiveTo[groupId])
	}

	friendsFileData, err := json.MarshalIndent(flaggedFriendsReport, "", "    ")
	if err != nil {
		fmt.Printf("[ERROR] Failed to process all flagged connections: %v\n", err)
	} else {
		if err := os.WriteFile("flagged_connections.json", friendsFileData, 0644); err != nil {
			fmt.Printf("[ERROR] Write failedl: %v\n", err)
		} else {
			fmt.Printf("[FILE] Dumped connections to Flagged Connections")
		}
	}

	printSummary := func(title string, users map[string]models.User) {
		fmt.Printf("\n%s\n", title)

		flaggedCount := 0
		associatedCount := 0

		fmt.Println("Flagged")
		for userId, user := range users {
			if flagData, isFlagged := flaggedUsers[userId]; isFlagged {
				flaggedCount += 1
				fmt.Printf("%s(%s)\n    Flags: %+v\n", user.UserName, userId, flagData)
			}
		}
		if flaggedCount == 0 {
			fmt.Printf("    No flagged accounts here")
		}

		fmt.Println("Associated")
		for userId, user := range users {
			if assocData, isAssociated := associationMap[userId]; isAssociated {
				associatedCount += 1
				fmt.Printf("%s(%s)\n", user.UserName, userId)
				for friendId, flagType := range assocData.FlaggedFriends {
					fmt.Printf("    [%s]: %s\n", friendId, flagType)
				}
			}
		}
		if associatedCount == 0 {
			fmt.Println("No associated accounts")
		}
	}

	printSummary("Both groups", result.InAllGroups)
	for groupId, groupName := range cfg.Groups {
		printSummary(fmt.Sprintf("%s(%s)", groupName, groupId), result.ExclusiveTo[groupId])
	}
}
