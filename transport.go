package isoautomate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// DefaultRPCWait is the default time to wait for a worker response (60s)
const DefaultRPCWait = 60 * time.Second

// Send transmits a generic command to the browser worker via Redis.
// It matches the Python _send method.
func (c *Client) Send(action string, args map[string]interface{}) (map[string]interface{}, error) {
	return c.SendWithTimeout(action, args, DefaultRPCWait)
}

// SendWithTimeout allows specifying a custom timeout (e.g., for release or heavy tasks).
func (c *Client) SendWithTimeout(action string, args map[string]interface{}, timeout time.Duration) (map[string]interface{}, error) {
	if c.Session == nil {
		return nil, NewBrowserError("Cannot perform action '%s': Browser session not acquired.", action)
	}

	// 1. Prepare Metadata
	taskID := uuid.New().Hex()
	resultKey := fmt.Sprintf("%sresult:%s", RedisPrefix, taskID)
	queue := fmt.Sprintf("%s%s:tasks", RedisPrefix, c.Session.WorkerName)

	// 2. Construct Payload
	// We use the struct for safety, but we might need to marshal it carefully to match Python's flat dict
	payload := TaskPayload{
		TaskID:     taskID,
		BrowserID:  c.Session.BrowserID,
		WorkerName: c.Session.WorkerName,
		Action:     action,
		Args:       args,
		ResultKey:  resultKey,
	}

	// 3. Handle Init Flags (Sent only on the first command)
	if !c.InitSent {
		if c.Session.Video {
			payload.Video = true
		}
		if c.Session.Record {
			payload.Record = true
		}
		if c.Session.ProfileID != "" {
			payload.ProfileID = c.Session.ProfileID
			payload.BrowserType = c.Session.BrowserType
		}
	}

	// Serialize Payload
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, NewBrowserError("Failed to serialize task payload: %v", err)
	}

	// 4. Send to Redis (RPUSH) with Retry
	err = c.executeWithRetry(func() error {
		return c.R.RPush(c.ctx, queue, data).Err()
	})
	if err != nil {
		return nil, err
	}

	// 5. Wait for Result (BLPOP) with Retry
	// We use the context for timeout to ensure we don't hang forever
	var resultRaw []string

	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()

	err = c.executeWithRetry(func() error {
		var rErr error
		resultRaw, rErr = c.R.BLPop(ctx, timeout, resultKey).Result()
		return rErr
	})

	if err != nil {
		if err == redis.Nil || err == context.DeadlineExceeded {
			return nil, NewBrowserError("Timeout waiting for worker response")
		}
		return nil, NewBrowserError("Redis RPC Error: %v", err)
	}

	// 6. Parse Response
	if len(resultRaw) < 2 {
		return nil, NewBrowserError("Invalid response from Redis")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(resultRaw[1]), &resp); err != nil {
		return nil, NewBrowserError("Failed to parse worker response: %v", err)
	}

	// Mark init as sent if successful
	c.InitSent = true

	// Cleanup result key (optional, but good hygiene if BLPOP didn't remove it, though BLPOP usually does pop it)
	// Redis BLPOP removes the item from the list. The key itself is a list.
	// We don't need to delete the list key explicitly if it's empty, Redis handles that.

	return resp, nil
}

// executeWithRetry mirrors the @redis_retry decorator in Python.
// It retries the operation up to 3 times with exponential backoff.
func (c *Client) executeWithRetry(op func() error) error {
	maxAttempts := 3
	backoffFactor := 0.2
	attempt := 0

	for {
		err := op()
		if err == nil {
			return nil
		}

		// Check if it's a Redis connection/timeout error
		// In Go, we check the error type or content
		// Ideally we only retry on network errors, but for simplicity we retry on most Redis errors except explicit logical ones
		if err != redis.Nil && attempt < maxAttempts {
			attempt++
			sleepTime := time.Duration(float64(time.Second) * backoffFactor * (1 << (attempt - 1))) // 0.2s, 0.4s, 0.8s
			time.Sleep(sleepTime)
			continue
		}
		return err
	}
}
