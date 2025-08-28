package distributed

import (
	"crypto/rand"
	"fmt"
	"os"
	"time"
)

// generateInstanceID creates a unique identifier for this application instance.
func generateInstanceID() string {
	hostname, _ := os.Hostname()
	pid := os.Getpid()
	
	// Add random bytes for uniqueness
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	
	return fmt.Sprintf("%s-%d-%x-%d", 
		hostname, pid, randomBytes, time.Now().Unix())
}

// redisKeys generates Redis keys for different data structures.
func redisKeys(prefix string) map[string]string {
	return map[string]string{
		"tokens":    prefix + ":tokens",
		"last":      prefix + ":last_refill", 
		"config":    prefix + ":config",
		"stats":     prefix + ":stats",
		"instances": prefix + ":instances",
		"locks":     prefix + ":locks",
	}
}

// timeToFloat converts time to float64 seconds for Redis storage.
func timeToFloat(t time.Time) float64 {
	return float64(t.UnixNano()) / 1e9
}

// floatToTime converts float64 seconds back to time.Time.
func floatToTime(f float64) time.Time {
	return time.Unix(0, int64(f*1e9))
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minFloat returns the minimum of two float64 values.
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// maxFloat returns the maximum of two float64 values.
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}