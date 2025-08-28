package distributed

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisFixedWindow implements distributed fixed window rate limiting using Redis.
type redisFixedWindow struct {
	config Config
	keys   map[string]string

	// Lua script for atomic fixed window operations
	checkAndIncrementScript *redis.Script
}

// newRedisFixedWindow creates a new Redis-based fixed window rate limiter.
func newRedisFixedWindow(config Config) (DistributedLimiter, error) {
	rfw := &redisFixedWindow{
		config: config,
		keys:   redisKeys(config.Key),
	}

	// Initialize Lua script
	rfw.checkAndIncrementScript = redis.NewScript(luaFixedWindowCheckAndIncrement)

	// Initialize in Redis
	if err := rfw.initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize fixed window: %w", err)
	}

	return rfw, nil
}

// initialize sets up the initial state in Redis.
func (rfw *redisFixedWindow) initialize(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, rfw.config.RedisTimeout)
	defer cancel()

	pipe := rfw.config.Redis.Pipeline()

	// Store configuration
	pipe.HSet(ctx, rfw.keys["config"], map[string]interface{}{
		"rate":            rfw.config.Rate,
		"burst":           rfw.config.Burst,
		"window_duration": time.Second.Nanoseconds(), // 1-second windows
	})
	pipe.Expire(ctx, rfw.keys["config"], rfw.config.KeyTTL)

	// Initialize stats
	pipe.HSet(ctx, rfw.keys["stats"], map[string]interface{}{
		"total_requests":   0,
		"allowed_requests": 0,
		"denied_requests":  0,
	})
	pipe.Expire(ctx, rfw.keys["stats"], rfw.config.KeyTTL)

	// Register this instance
	pipe.SAdd(ctx, rfw.keys["instances"], rfw.config.InstanceID)
	pipe.Expire(ctx, rfw.keys["instances"], rfw.config.KeyTTL)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return &RedisError{"initialize", err}
	}

	return nil
}

// getWindowKey returns the Redis key for the current time window.
func (rfw *redisFixedWindow) getWindowKey(t time.Time) string {
	// Create 1-second windows
	windowStart := t.Unix()
	return fmt.Sprintf("%s:window:%d", rfw.keys["tokens"], windowStart)
}

// Allow reports whether an event may happen now.
func (rfw *redisFixedWindow) Allow(ctx context.Context) bool {
	return rfw.AllowN(ctx, 1)
}

// AllowN reports whether n events may happen now.
func (rfw *redisFixedWindow) AllowN(ctx context.Context, n int) bool {
	if n <= 0 {
		return true
	}

	ctx, cancel := context.WithTimeout(ctx, rfw.config.RedisTimeout)
	defer cancel()

	now := time.Now()
	windowKey := rfw.getWindowKey(now)

	result, err := rfw.checkAndIncrementScript.Run(ctx, rfw.config.Redis, []string{
		windowKey,
		rfw.keys["stats"],
	},
		n,
		int64(rfw.config.Rate),     // Max requests per window
		int(time.Second.Seconds()), // Window TTL
	).Result()

	if err != nil {
		// Fallback to local limiter if configured
		if rfw.config.FallbackToLocal && rfw.config.LocalLimiter != nil {
			return rfw.config.LocalLimiter.AllowN(n)
		}
		return false
	}

	allowed, ok := result.(int64)
	return ok && allowed == 1
}

// Wait blocks until an event can happen.
func (rfw *redisFixedWindow) Wait(ctx context.Context) error {
	return rfw.WaitN(ctx, 1)
}

// WaitN blocks until n events can happen.
func (rfw *redisFixedWindow) WaitN(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}

	// For fixed window, we need to wait for the next window if current is full
	ticker := time.NewTicker(rfw.config.RefreshInterval)
	defer ticker.Stop()

	for {
		if rfw.AllowN(ctx, n) {
			return nil
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Reserve returns information about when to act for n events.
func (rfw *redisFixedWindow) Reserve(ctx context.Context, n int) (*Reservation, error) {
	if n <= 0 {
		return &Reservation{OK: true, Delay: 0, Tokens: 0}, nil
	}

	// Try to allow immediately
	if rfw.AllowN(ctx, n) {
		return &Reservation{
			OK:         true,
			Delay:      0,
			Tokens:     n,
			AllowedAt:  time.Now(),
			InstanceID: rfw.config.InstanceID,
		}, nil
	}

	// Calculate delay to next window
	now := time.Now()
	nextWindowStart := now.Truncate(time.Second).Add(time.Second)
	delay := nextWindowStart.Sub(now)

	return &Reservation{
		OK:         false,
		Delay:      delay,
		Tokens:     n,
		AllowedAt:  nextWindowStart,
		InstanceID: rfw.config.InstanceID,
	}, nil
}

// SetRate changes the rate limit across all instances.
func (rfw *redisFixedWindow) SetRate(ctx context.Context, rate float64) error {
	if rate <= 0 {
		return &ConfigError{"rate must be positive"}
	}

	ctx, cancel := context.WithTimeout(ctx, rfw.config.RedisTimeout)
	defer cancel()

	err := rfw.config.Redis.HSet(ctx, rfw.keys["config"], "rate", rate).Err()
	if err != nil {
		return &RedisError{"set_rate", err}
	}

	rfw.config.Rate = rate
	return nil
}

// SetBurst changes the burst capacity across all instances.
func (rfw *redisFixedWindow) SetBurst(ctx context.Context, burst int) error {
	if burst <= 0 {
		return &ConfigError{"burst must be positive"}
	}

	ctx, cancel := context.WithTimeout(ctx, rfw.config.RedisTimeout)
	defer cancel()

	err := rfw.config.Redis.HSet(ctx, rfw.keys["config"], "burst", burst).Err()
	if err != nil {
		return &RedisError{"set_burst", err}
	}

	rfw.config.Burst = burst
	return nil
}

// Stats returns current limiter statistics.
func (rfw *redisFixedWindow) Stats(ctx context.Context) (*Stats, error) {
	ctx, cancel := context.WithTimeout(ctx, rfw.config.RedisTimeout)
	defer cancel()

	pipe := rfw.config.Redis.Pipeline()

	configCmd := pipe.HGetAll(ctx, rfw.keys["config"])
	instancesCmd := pipe.SMembers(ctx, rfw.keys["instances"])
	statsCmd := pipe.HGetAll(ctx, rfw.keys["stats"])

	// Get current window count
	now := time.Now()
	windowKey := rfw.getWindowKey(now)
	currentWindowCmd := pipe.Get(ctx, windowKey)

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, &RedisError{"stats", err}
	}

	configMap := configCmd.Val()
	rate, _ := strconv.ParseFloat(configMap["rate"], 64)
	burst, _ := strconv.Atoi(configMap["burst"])

	instances := instancesCmd.Val()

	statsMap := statsCmd.Val()
	totalRequests, _ := strconv.ParseInt(statsMap["total_requests"], 10, 64)
	allowedRequests, _ := strconv.ParseInt(statsMap["allowed_requests"], 10, 64)
	deniedRequests, _ := strconv.ParseInt(statsMap["denied_requests"], 10, 64)

	currentWindowCount, _ := strconv.ParseFloat(currentWindowCmd.Val(), 64)
	remainingCapacity := rate - currentWindowCount

	return &Stats{
		Rate:            rate,
		Burst:           burst,
		Tokens:          maxFloat(0, remainingCapacity), // Remaining capacity in current window
		LastRefill:      now.Truncate(time.Second),      // Window start time
		TotalRequests:   totalRequests,
		AllowedRequests: allowedRequests,
		DeniedRequests:  deniedRequests,
		ActiveInstances: instances,
	}, nil
}

// Reset clears the limiter state.
func (rfw *redisFixedWindow) Reset(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, rfw.config.RedisTimeout)
	defer cancel()

	// Clear all window keys (this is a simplified approach)
	// In production, you might want to scan and delete window keys
	keys := make([]string, 0, len(rfw.keys))
	for _, key := range rfw.keys {
		keys = append(keys, key)
	}

	err := rfw.config.Redis.Del(ctx, keys...).Err()
	if err != nil {
		return &RedisError{"reset", err}
	}

	return rfw.initialize(ctx)
}

// Close cleanly shuts down the limiter.
func (rfw *redisFixedWindow) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), rfw.config.RedisTimeout)
	defer cancel()

	return rfw.config.Redis.SRem(ctx, rfw.keys["instances"], rfw.config.InstanceID).Err()
}

// Lua script for fixed window operations
const luaFixedWindowCheckAndIncrement = `
-- KEYS[1]: current window key  
-- KEYS[2]: stats key
-- ARGV[1]: requests count
-- ARGV[2]: rate limit (max requests per window)
-- ARGV[3]: window TTL (seconds)

local window_key = KEYS[1]
local stats_key = KEYS[2]

local requests = tonumber(ARGV[1])
local max_requests = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])

-- Get current count in this window
local current_count = tonumber(redis.call('GET', window_key) or "0")

-- Check if we can allow this request
if current_count + requests <= max_requests then
    -- Increment the counter
    local new_count = redis.call('INCRBY', window_key, requests)
    
    -- Set TTL if this is a new window
    if new_count == requests then
        redis.call('EXPIRE', window_key, ttl + 1) -- Add 1 second buffer
    end
    
    -- Update stats
    redis.call('HINCRBY', stats_key, 'total_requests', requests)
    redis.call('HINCRBY', stats_key, 'allowed_requests', requests)
    
    return 1 -- allowed
else
    -- Update stats
    redis.call('HINCRBY', stats_key, 'total_requests', requests)
    redis.call('HINCRBY', stats_key, 'denied_requests', requests)
    
    return 0 -- denied
end
`
