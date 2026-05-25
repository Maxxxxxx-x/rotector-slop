package roblox

import (
	"net/http"
	"time"

	"github.com/Maxxxxxx-x/rotector-slop/api"
)

var (
	rbxClient = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	groupLimiter  = api.NewBucket(200*time.Millisecond, 10)
	friendLimiter = api.NewBucket(200*time.Millisecond, 10)
)
