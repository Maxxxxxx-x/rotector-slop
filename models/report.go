package models

type GroupReport struct {
	Flagged    map[string]ProcessedFlagData `json:"flagged"`
	Associated map[string]AssociatedUser    `json:"associated"`
}
