package utils

import (
	"math"
	"net/http"
	"strconv"
	"time"
)

func CalculateRetryDelay(headerVal string, attempt int) time.Duration {
	if headerVal != "" {
		if seconds, err := strconv.Atoi(headerVal); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}

		if targetTime, err := http.ParseTime(headerVal); err == nil {
			if delay := time.Until(targetTime); delay > 0 {
				return delay
			}
		}
	}

	return time.Duration(math.Pow(2, float64(attempt+2))) * time.Second
}
