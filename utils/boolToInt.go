package utils

func BoolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
