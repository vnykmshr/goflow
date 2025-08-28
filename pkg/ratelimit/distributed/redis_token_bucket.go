package distributed

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisTokenBucket implements distributed token bucket using Redis.
type redisTokenBucket struct {
	config Config
	keys   map[string]string
	
	// Lua script for atomic token bucket operations
	tryConsumeScript *redis.Script
	refillScript     *redis.Script
	statsScript      *redis.Script
}

// newRedisTokenBucket creates a new Redis-based distributed token bucket.
func newRedisTokenBucket(config Config) (DistributedLimiter, error) {
	rtb := &redisTokenBucket{
		config: config,
		keys:   redisKeys(config.Key),
	}

	// Initialize Lua scripts for atomic operations
	rtb.tryConsumeScript = redis.NewScript(luaTryConsume)
	rtb.refillScript = redis.NewScript(luaRefill)  
	rtb.statsScript = redis.NewScript(luaStats)

	// Initialize the bucket in Redis
	if err := rtb.initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize token bucket: %w", err)
	}

	return rtb, nil
}

// initialize sets up the initial state in Redis.
func (rtb *redisTokenBucket) initialize(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, rtb.config.RedisTimeout)
	defer cancel()

	pipe := rtb.config.Redis.Pipeline()
	
	// Set initial values if they don't exist
	pipe.SetNX(ctx, rtb.keys["tokens"], float64(rtb.config.Burst), rtb.config.KeyTTL)
	pipe.SetNX(ctx, rtb.keys["last"], timeToFloat(time.Now()), rtb.config.KeyTTL)
	
	// Store configuration
	pipe.HSet(ctx, rtb.keys["config"], map[string]interface{}{
		"rate":  rtb.config.Rate,
		"burst": rtb.config.Burst,
	})
	pipe.Expire(ctx, rtb.keys["config"], rtb.config.KeyTTL)
	
	// Register this instance
	pipe.SAdd(ctx, rtb.keys["instances"], rtb.config.InstanceID)
	pipe.Expire(ctx, rtb.keys["instances"], rtb.config.KeyTTL)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return &RedisError{"initialize", err}
	}

	return nil
}

// Allow reports whether an event may happen now.
func (rtb *redisTokenBucket) Allow(ctx context.Context) bool {
	return rtb.AllowN(ctx, 1)
}

// AllowN reports whether n events may happen now.
func (rtb *redisTokenBucket) AllowN(ctx context.Context, n int) bool {
	if n <= 0 {
		return true
	}

	reservation, err := rtb.Reserve(ctx, n)
	if err != nil {
		// Fallback to local limiter if configured
		if rtb.config.FallbackToLocal && rtb.config.LocalLimiter != nil {
			return rtb.config.LocalLimiter.AllowN(n)
		}
		return false
	}

	return reservation.OK && reservation.Delay <= 0
}

// Wait blocks until an event can happen.
func (rtb *redisTokenBucket) Wait(ctx context.Context) error {
	return rtb.WaitN(ctx, 1)
}

// WaitN blocks until n events can happen.
func (rtb *redisTokenBucket) WaitN(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}

	reservation, err := rtb.Reserve(ctx, n)
	if err != nil {
		// Fallback to local limiter if configured
		if rtb.config.FallbackToLocal && rtb.config.LocalLimiter != nil {
			return rtb.config.LocalLimiter.WaitN(ctx, n)
		}
		return err
	}

	if !reservation.OK {
		return fmt.Errorf("rate limit exceeded")
	}

	if reservation.Delay > 0 {
		timer := time.NewTimer(reservation.Delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// Reserve returns information about when to act for n events.
func (rtb *redisTokenBucket) Reserve(ctx context.Context, n int) (*Reservation, error) {
	if n <= 0 {
		return &Reservation{OK: true, Delay: 0, Tokens: 0}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, rtb.config.RedisTimeout)
	defer cancel()

	now := time.Now()
	result, err := rtb.tryConsumeScript.Run(ctx, rtb.config.Redis, []string{
		rtb.keys["tokens"],
		rtb.keys["last"],
		rtb.keys["config"],
	}, 
		n,                           // tokens requested
		timeToFloat(now),           // current time
		rtb.config.Rate,            // refill rate
		rtb.config.Burst,           // max capacity
	).Result()

	if err != nil {
		return nil, &RedisError{"reserve", err}
	}

	// Parse Lua script result: [allowed, tokens_after, delay_seconds]
	resultSlice, ok := result.([]interface{})
	if !ok || len(resultSlice) != 3 {
		return nil, fmt.Errorf("invalid script result")
	}

	allowed, _ := resultSlice[0].(int64)
	tokensAfter, _ := resultSlice[1].(string)
	delaySeconds, _ := resultSlice[2].(string)

	_, _ = strconv.ParseFloat(tokensAfter, 64) // tokens after operation
	delay, _ := strconv.ParseFloat(delaySeconds, 64)

	reservation := &Reservation{
		OK:         allowed == 1,
		Delay:      time.Duration(delay * float64(time.Second)),
		Tokens:     n,
		AllowedAt:  now.Add(time.Duration(delay * float64(time.Second))),
		InstanceID: rtb.config.InstanceID,
	}

	return reservation, nil
}

// SetRate changes the rate limit across all instances.
func (rtb *redisTokenBucket) SetRate(ctx context.Context, rate float64) error {
	if rate <= 0 {
		return &ConfigError{"rate must be positive"}
	}

	ctx, cancel := context.WithTimeout(ctx, rtb.config.RedisTimeout)
	defer cancel()

	err := rtb.config.Redis.HSet(ctx, rtb.keys["config"], "rate", rate).Err()
	if err != nil {
		return &RedisError{"set_rate", err}
	}

	rtb.config.Rate = rate
	return nil
}

// SetBurst changes the burst capacity across all instances.
func (rtb *redisTokenBucket) SetBurst(ctx context.Context, burst int) error {
	if burst <= 0 {
		return &ConfigError{"burst must be positive"}
	}

	ctx, cancel := context.WithTimeout(ctx, rtb.config.RedisTimeout)
	defer cancel()

	err := rtb.config.Redis.HSet(ctx, rtb.keys["config"], "burst", burst).Err()
	if err != nil {
		return &RedisError{"set_burst", err}
	}

	rtb.config.Burst = burst
	return nil
}

// Stats returns current limiter statistics.
func (rtb *redisTokenBucket) Stats(ctx context.Context) (*Stats, error) {
	ctx, cancel := context.WithTimeout(ctx, rtb.config.RedisTimeout)
	defer cancel()

	pipe := rtb.config.Redis.Pipeline()
	
	tokensCmd := pipe.Get(ctx, rtb.keys["tokens"])
	lastCmd := pipe.Get(ctx, rtb.keys["last"])
	configCmd := pipe.HGetAll(ctx, rtb.keys["config"])
	instancesCmd := pipe.SMembers(ctx, rtb.keys["instances"])
	statsCmd := pipe.HGetAll(ctx, rtb.keys["stats"])

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, &RedisError{"stats", err}
	}

	tokens, _ := strconv.ParseFloat(tokensCmd.Val(), 64)
	lastRefill := floatToTime(func() float64 {
		if val, err := strconv.ParseFloat(lastCmd.Val(), 64); err == nil {
			return val
		}
		return 0
	}())

	configMap := configCmd.Val()
	rate, _ := strconv.ParseFloat(configMap["rate"], 64)
	burst, _ := strconv.Atoi(configMap["burst"])

	instances := instancesCmd.Val()
	
	statsMap := statsCmd.Val()
	totalRequests, _ := strconv.ParseInt(statsMap["total_requests"], 10, 64)
	allowedRequests, _ := strconv.ParseInt(statsMap["allowed_requests"], 10, 64)
	deniedRequests, _ := strconv.ParseInt(statsMap["denied_requests"], 10, 64)

	return &Stats{
		Rate:            rate,
		Burst:           burst,
		Tokens:          tokens,
		LastRefill:      lastRefill,
		TotalRequests:   totalRequests,
		AllowedRequests: allowedRequests,
		DeniedRequests:  deniedRequests,
		ActiveInstances: instances,
	}, nil
}

// Reset clears the limiter state.
func (rtb *redisTokenBucket) Reset(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, rtb.config.RedisTimeout)
	defer cancel()

	// Delete all keys related to this limiter
	keys := make([]string, 0, len(rtb.keys))
	for _, key := range rtb.keys {
		keys = append(keys, key)
	}

	err := rtb.config.Redis.Del(ctx, keys...).Err()
	if err != nil {
		return &RedisError{"reset", err}
	}

	// Reinitialize
	return rtb.initialize(ctx)
}

// Close cleanly shuts down the limiter.
func (rtb *redisTokenBucket) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), rtb.config.RedisTimeout)
	defer cancel()

	// Remove this instance from the active instances set
	return rtb.config.Redis.SRem(ctx, rtb.keys["instances"], rtb.config.InstanceID).Err()
}

// Lua scripts for atomic operations
const luaTryConsume = `
-- KEYS[1]: tokens key
-- KEYS[2]: last_refill key  
-- KEYS[3]: config key
-- ARGV[1]: tokens requested
-- ARGV[2]: current time
-- ARGV[3]: refill rate
-- ARGV[4]: max capacity

local tokens_key = KEYS[1]
local last_key = KEYS[2] 
local config_key = KEYS[3]

local requested = tonumber(ARGV[1])
local now = tonumber(ARGV[2])
local rate = tonumber(ARGV[3])
local capacity = tonumber(ARGV[4])

-- Get current state
local tokens = tonumber(redis.call('GET', tokens_key) or capacity)
local last_refill = tonumber(redis.call('GET', last_key) or now)

-- Calculate tokens to add based on time passed
local time_passed = math.max(0, now - last_refill)
local tokens_to_add = time_passed * rate
local new_tokens = math.min(capacity, tokens + tokens_to_add)

-- Check if we can satisfy the request
if new_tokens >= requested then
    -- Consume tokens
    new_tokens = new_tokens - requested
    
    -- Update Redis state
    redis.call('SET', tokens_key, tostring(new_tokens))
    redis.call('SET', last_key, tostring(now))
    
    -- Update stats
    redis.call('HINCRBY', KEYS[3]:gsub(':config', ':stats'), 'total_requests', 1)
    redis.call('HINCRBY', KEYS[3]:gsub(':config', ':stats'), 'allowed_requests', 1)
    
    return {1, tostring(new_tokens), "0"}
else
    -- Calculate delay needed
    local tokens_needed = requested - new_tokens
    local delay_seconds = tokens_needed / rate
    
    -- Update stats  
    redis.call('HINCRBY', KEYS[3]:gsub(':config', ':stats'), 'total_requests', 1)
    redis.call('HINCRBY', KEYS[3]:gsub(':config', ':stats'), 'denied_requests', 1)
    
    return {0, tostring(new_tokens), tostring(delay_seconds)}
end
`

const luaRefill = `
-- KEYS[1]: tokens key
-- KEYS[2]: last_refill key
-- ARGV[1]: current time
-- ARGV[2]: refill rate  
-- ARGV[3]: max capacity

local tokens_key = KEYS[1]
local last_key = KEYS[2]

local now = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local capacity = tonumber(ARGV[3])

local tokens = tonumber(redis.call('GET', tokens_key) or capacity)
local last_refill = tonumber(redis.call('GET', last_key) or now)

local time_passed = math.max(0, now - last_refill)
local tokens_to_add = time_passed * rate
local new_tokens = math.min(capacity, tokens + tokens_to_add)

redis.call('SET', tokens_key, tostring(new_tokens))
redis.call('SET', last_key, tostring(now))

return new_tokens
`

const luaStats = `
-- KEYS[1]: tokens key
-- KEYS[2]: last_refill key
-- KEYS[3]: config key
-- KEYS[4]: stats key

return {
    redis.call('GET', KEYS[1]) or "0",
    redis.call('GET', KEYS[2]) or "0", 
    redis.call('HGETALL', KEYS[3]),
    redis.call('HGETALL', KEYS[4])
}
`