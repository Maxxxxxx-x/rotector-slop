package utils

import "encoding/json"

func ToJson(v any) string {
	if v == nil {
		return "{}"
	}

	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
