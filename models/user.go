package models

type User struct {
	UserName string
	RoleName string
	Flags    FlagResponse
	Friends  map[string]Friend
}

type AssociatedUser struct {
	FlaggedFriends map[string]string
}
