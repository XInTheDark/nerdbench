package bench

import "time"

var profileBudgets = map[string]time.Duration{
	"smoke":    1 * time.Second,
	"quick":    5 * time.Second,
	"standard": 18 * time.Second,
	"extended": 75 * time.Second,
}

func profileDuration(profile string) time.Duration {
	if d, ok := profileBudgets[profile]; ok {
		return d
	}
	return profileBudgets["standard"]
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
