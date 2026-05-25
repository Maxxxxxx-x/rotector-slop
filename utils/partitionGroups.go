package utils

import (
	"fmt"

	"github.com/Maxxxxxx-x/rotector-slop/models"
)

func PartiitionGroups(memberList map[string]map[string]models.User) (map[string][]string, map[string]models.User, error) {
	if memberList == nil {
		return nil, nil, fmt.Errorf("Memberlist is null")
	}

	if len(memberList) == 0 {
		return make(map[string][]string), make(map[string]models.User), nil
	}

	totalGroups := len(memberList)
	estimatedUsers := 0
	for _, m := range memberList {
		estimatedUsers += len(m)
	}

	type Tracker struct {
		user     models.User
		count    int
		firstGrp string
	}

	users := make(map[string]*Tracker, estimatedUsers/2)

	for groupId, members := range memberList {
		for userId, user := range members {
			t, exists := users[userId]
			if !exists {
				users[userId] = &Tracker{
					user:     user,
					count:    1,
					firstGrp: groupId,
				}
				continue
			}
			t.count += 1
		}
	}

	grpToUsrs := make(map[string][]string, totalGroups+1)
	for groupId := range memberList {
		grpToUsrs[groupId] = make([]string, 0, 16)
	}
	grpToUsrs["Both"] = make([]string, 0, 16)

	uniqueUsers := make(map[string]models.User, len(users))

	for userId, t := range users {
		uniqueUsers[userId] = t.user

		if t.count == 1 {
			grpToUsrs[t.firstGrp] = append(grpToUsrs[t.firstGrp], userId)
			continue
		}
		grpToUsrs["Both"] = append(grpToUsrs["Both"], userId)
	}

	return grpToUsrs, uniqueUsers, nil
}
