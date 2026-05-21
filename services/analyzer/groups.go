package analyzer

import (
	"github.com/Maxxxxxx-x/rotector-slop/models"
)

type GroupAnalyzeResult struct {
	InAllGroups map[string]models.User
	ExclusiveTo map[string]map[string]models.User
}

func AnalyzeGroups(groupMembers map[string]map[string]models.User) GroupAnalyzeResult {
	totalGroups := len(groupMembers)
	result := GroupAnalyzeResult{
		InAllGroups: make(map[string]models.User),
		ExclusiveTo: make(map[string]map[string]models.User, totalGroups),
	}

	if totalGroups == 0 {
		return result
	}

	type Tracker struct {
		user         models.User
		count        int
		firstGroupId string
	}

	estimatedTotalUsers := 0
	for _, userMap := range groupMembers {
		estimatedTotalUsers += len(userMap)
	}
	users := make(map[string]*Tracker, estimatedTotalUsers/2)

	for groupId, members := range groupMembers {
		for userId, member := range members {
			tracker, exists := users[userId]
			if !exists {
				users[userId] = &Tracker{
					user:         member,
					count:        1,
					firstGroupId: groupId,
				}
				continue
			}
			tracker.count += 1
		}
	}

	for userId, tracker := range users {
		if tracker.count == totalGroups {
			result.InAllGroups[userId] = tracker.user
			continue
		}

		if tracker.count == 1 {
			groupId := tracker.firstGroupId
			if result.ExclusiveTo[groupId] == nil {
				result.ExclusiveTo[groupId] = make(map[string]models.User)
			}
			result.ExclusiveTo[groupId][userId] = tracker.user
		}
	}

	return result
}
