package models

import "encoding/json"

type FlagReason struct {
	Message    string   `json:"message"`
	Confidence float32  `json:"confidence"`
	Evidence   []string `json:"evidence"`
}

type Reviewer struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type MembershipBadge struct {
	Tier        int `json:"tier"`
	BadgeDesign int `json:"badgeDesign"`
	IconDesign  int `json:"iconDesign"`
	TextDesign  int `json:"textDesign"`
}

type FlagData struct {
	Id                   int                   `json:"id"`
	FlagType             int                   `json:"flagType"`
	Category             int                   `json:"category"`
	Confidence           float64               `json:"confidence"`
	Reasons              map[string]FlagReason `json:"reasons"`
	Reviewer             Reviewer              `json:"reviewer"`
	EngineVersion        string                `json:"engineVersion"`
	VersionCompatibility string                `json:"versionCompatibility"`
	IsReportable         bool                  `json:"isReportable"`
	IsLocked             bool                  `json:"isLocked"`
	QueuedAt             int64                 `json:"queuedAt"`
	Processed            bool                  `json:"processed"`
	ProcessedAt          int64                 `json:"processedAt"`
	LastUpdated          int64                 `json:"lastUpdated"`
	MembershipBadge      MembershipBadge       `json:"membershipBadge"`
}

type ProcessedFlagData struct {
	Id           int                   `json:"id"`
	FlagType     string                `json:"flagType"`
	Category     string                `json:"category"`
	Confidence   float64               `json:"confidence"`
	Reasons      map[string]FlagReason `json:"reasons"`
	Reviewer     string                `json:"reviewer"`
	IsReportable bool                  `json:"isReportable"`
	IsLocked     bool                  `json:"isLocked"`
	QueuedAt     string                `json:"queuedAt"`
	ProcessedAt  string                `json:"processedAt"`
	LastUpdated  string                `json:"lastUpdated"`
}

type FlagResponse struct {
	Success bool     `json:"success"`
	Data    FlagData `json:"data"`
}

type BatchFlagResponse struct {
	Success bool                       `json:"success"`
	Data    map[string]json.RawMessage `json:"data"`
}
