package distributed

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisSlidingWindow implements distributed sliding window rate limiting using Redis.
type redisSlidingWindow struct {
	config Config
	keys   map[string]string

	// Lua script for atomic sliding window operations
	checkAndIncrementScript *redis.Script
	cleanupScript           *redis.Script
}

// newRedisSlidingWindow creates a new Redis-based sliding window rate limiter.
func newRedisSlidingWindow(config Config) (Limiter, error) {
	rsw := &redisSlidingWindow{
		config: config,
		keys:   redisKeys(config.Key),
	}

	// Initialize Lua scripts
	rsw.checkAndIncrementScript = redis.NewScript(luaSlidingWindowCheckAndIncrement)
	rsw.cleanupScript = redis.NewScript(luaSlidingWindowCleanup)

	// Initialize in Redis
	if err := rsw.initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize sliding window: %w", err)
	}

	return rsw, nil
}

// initialize sets up the initial state in Redis.
func (rsw *redisSlidingWindow) initialize(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, rsw.config.RedisTimeout)
	defer cancel()

	pipe := rsw.config.Redis.Pipeline()

	// Store configuration
	pipe.HSet(ctx, rsw.keys["config"], map[string]interface{}{
		"rate":        rsw.config.Rate,
		"burst":       rsw.config.Burst,
		"window_size": time.Second.Nanoseconds(), // 1-second sliding window
	})
	pipe.Expire(ctx, rsw.keys["config"], rsw.config.KeyTTL)

	// Initialize stats
	pipe.HSet(ctx, rsw.keys["stats"], map[string]interface{}{
		"total_requests":   0,
		"allowed_requests": 0,
		"denied_requests":  0,
	})
	pipe.Expire(ctx, rsw.keys["stats"], rsw.config.KeyTTL)

	// Register this instance
	pipe.SAdd(ctx, rsw.keys["instances"], rsw.config.InstanceID)
	pipe.Expire(ctx, rsw.keys["instances"], rsw.config.KeyTTL)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return &RedisError{"initialize", err}
	}

	return nil
}

// Allow reports whether an event may happen now.
func (rsw *redisSlidingWindow) Allow(ctx context.Context) bool {
	return rsw.AllowN(ctx, 1)
}

// AllowN reports whether n events may happen now.
func (rsw *redisSlidingWindow) AllowN(ctx context.Context, n int) bool {
	if n <= 0 {
		return true
	}

	ctx, cancel := context.WithTimeout(ctx, rsw.config.RedisTimeout)
	defer cancel()

	now := time.Now().UnixNano()
	windowStart := now - time.Second.Nanoseconds() // 1-second window

	result, err := rsw.checkAndIncrementScript.Run(ctx, rsw.config.Redis, []string{
		rsw.keys["tokens"], // Use tokens key as the sliding window counter key
		rsw.keys["stats"],
	},
		now,
		windowStart,
		n,
		int64(rsw.config.Rate), // Max requests per second
	).Result()

	if err != nil {
		// Fallback to local limiter if configured
		if rsw.config.FallbackToLocal && rsw.config.LocalLimiter != nil {
			return rsw.config.LocalLimiter.AllowN(n)
		}
		return false
	}

	allowed, ok := result.(int64)
	return ok && allowed == 1
}

// Wait blocks until an event can happen.
func (rsw *redisSlidingWindow) Wait(ctx context.Context) error {
	return rsw.WaitN(ctx, 1)
}

// WaitN blocks until n events can happen.
func (rsw *redisSlidingWindow) WaitN(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}

	// For sliding window, we use a simple retry strategy
	ticker := time.NewTicker(rsw.config.RefreshInterval)
	defer ticker.Stop()

	for {
		if rsw.AllowN(ctx, n) {
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
func (rsw *redisSlidingWindow) Reserve(ctx context.Context, n int) (*Reservation, error) {
	if n <= 0 {
		return &Reservation{OK: true, Delay: 0, Tokens: 0}, nil
	}

	// For sliding window, if we can't allow now, estimate delay
	if rsw.AllowN(ctx, n) {
		return &Reservation{
			OK:         true,
			Delay:      0,
			Tokens:     n,
			AllowedAt:  time.Now(),
			InstanceID: rsw.config.InstanceID,
		}, nil
	}

	// Estimate delay based on refill rate
	delay := time.Duration(float64(n)/rsw.config.Rate) * time.Second
	return &Reservation{
		OK:         false,
		Delay:      delay,
		Tokens:     n,
		AllowedAt:  time.Now().Add(delay),
		InstanceID: rsw.config.InstanceID,
	}, nil
}

// SetRate changes the rate limit across all instances.
func (rsw *redisSlidingWindow) SetRate(ctx context.Context, rate float64) error {
	if rate <= 0 {
		return &ConfigError{"rate must be positive"}
	}

	ctx, cancel := context.WithTimeout(ctx, rsw.config.RedisTimeout)
	defer cancel()

	err := rsw.config.Redis.HSet(ctx, rsw.keys["config"], "rate", rate).Err()
	if err != nil {
		return &RedisError{"set_rate", err}
	}

	rsw.config.Rate = rate
	return nil
}

// SetBurst changes the burst capacity across all instances.
func (rsw *redisSlidingWindow) SetBurst(ctx context.Context, burst int) error {
	if burst <= 0 {
		return &ConfigError{"burst must be positive"}
	}

	ctx, cancel := context.WithTimeout(ctx, rsw.config.RedisTimeout)
	defer cancel()

	err := rsw.config.Redis.HSet(ctx, rsw.keys["config"], "burst", burst).Err()
	if err != nil {
		return &RedisError{"set_burst", err}
	}

	rsw.config.Burst = burst
	return nil
}

// Stats returns current limiter statistics.
func (rsw *redisSlidingWindow) Stats(ctx context.Context) (*Stats, error) {
	ctx, cancel := context.WithTimeout(ctx, rsw.config.RedisTimeout)
	defer cancel()

	pipe := rsw.config.Redis.Pipeline()

	configCmd := pipe.HGetAll(ctx, rsw.keys["config"])
	instancesCmd := pipe.SMembers(ctx, rsw.keys["instances"])
	statsCmd := pipe.HGetAll(ctx, rsw.keys["stats"])

	// Get current window count
	now := time.Now().UnixNano()
	windowStart := now - time.Second.Nanoseconds()
	windowCountCmd := pipe.ZCount(ctx, rsw.keys["tokens"],
		strconv.FormatInt(windowStart, 10),
		strconv.FormatInt(now, 10))

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

	windowCount := float64(windowCountCmd.Val())

	return &Stats{
		Rate:            rate,
		Burst:           burst,
		Tokens:          float64(rsw.config.Burst) - windowCount, // Remaining capacity
		LastRefill:      time.Now(),
		TotalRequests:   totalRequests,
		AllowedRequests: allowedRequests,
		DeniedRequests:  deniedRequests,
		ActiveInstances: instances,
	}, nil
}

// Reset clears the limiter state.
func (rsw *redisSlidingWindow) Reset(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, rsw.config.RedisTimeout)
	defer cancel()

	keys := make([]string, 0, len(rsw.keys))
	for _, key := range rsw.keys {
		keys = append(keys, key)
	}

	err := rsw.config.Redis.Del(ctx, keys...).Err()
	if err != nil {
		return &RedisError{"reset", err}
	}

	return rsw.initialize(ctx)
}

// Close cleanly shuts down the limiter.
func (rsw *redisSlidingWindow) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), rsw.config.RedisTimeout)
	defer cancel()

	return rsw.config.Redis.SRem(ctx, rsw.keys["instances"], rsw.config.InstanceID).Err()
}

// Lua scripts for sliding window operations
const luaSlidingWindowCheckAndIncrement = `
-- KEYS[1]: sliding window key (sorted set)
-- KEYS[2]: stats key
-- ARGV[1]: current time (nanoseconds)
-- ARGV[2]: window start time (nanoseconds) 
-- ARGV[3]: requests count
-- ARGV[4]: rate limit (max requests per window)

local window_key = KEYS[1]
local stats_key = KEYS[2]

local now = tonumber(ARGV[1])
local window_start = tonumber(ARGV[2])
local requests = tonumber(ARGV[3])
local max_requests = tonumber(ARGV[4])

-- Clean up old entries outside the window
redis.call('ZREMRANGEBYSCORE', window_key, '-inf', window_start - 1)

-- Count current requests in the window
local current_count = redis.call('ZCARD', window_key)

-- Check if we can allow this request
if current_count + requests <= max_requests then
    -- Add the new requests to the window
    for i = 1, requests do
        -- Use unique scores to handle multiple requests at the same timestamp
        local score = now + i - 1
        redis.call('ZADD', window_key, score, score .. ':' .. math.random(1000000))
    end
    
    -- Set expiry for cleanup
    redis.call('EXPIRE', window_key, 2)
    
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

const luaSlidingWindowCleanup = `
-- KEYS[1]: sliding window key
-- ARGV[1]: cutoff time (nanoseconds)

local window_key = KEYS[1]
local cutoff_time = tonumber(ARGV[1])

-- Remove entries older than cutoff
return redis.call('ZREMRANGEBYSCORE', window_key, '-inf', cutoff_time)
`
