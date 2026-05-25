package models

type User struct {
	Name    string
	Role    string
	Flags   ProcessedFlagData
	Friends []string
}

type AssociatedUser struct {
	FlaggedFriends map[string]string
}
