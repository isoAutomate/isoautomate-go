package isoautomate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Acquire reserves a browser session using atomic Lua scripting.
func (c *Client) Acquire(browserType string, video bool, profile interface{}, record bool) (map[string]interface{}, error) {
	// 1. Handle Profile Logic
	var profileID string
	if profile != nil {
		if pStr, ok := profile.(string); ok {
			profileID = pStr
		} else if pBool, ok := profile.(bool); ok && pBool {
			// Auto-generate or load default profile ID
			cwd, _ := os.Getwd()
			profileStore := filepath.Join(cwd, ".iso_profiles")
			_ = os.MkdirAll(profileStore, 0755)

			idFile := filepath.Join(profileStore, "default_profile.id")
			if data, err := os.ReadFile(idFile); err == nil {
				profileID = string(data)
			} else {
				profileID = fmt.Sprintf("user_%s", uuid.New().Hex()[:8])
				_ = os.WriteFile(idFile, []byte(profileID), 0644)
			}
		}
	}

	c.InitSent = false

	// 2. The Lua Script (Exact copy of Python logic)
	luaScript := `
	local workers = redis.call('SMEMBERS', KEYS[1])
	for i = #workers, 2, -1 do
		local j = math.random(i)
		workers[i], workers[j] = workers[j], workers[i]
	end
	
	for _, worker in ipairs(workers) do
		local free_key = ARGV[1] .. worker .. ':' .. ARGV[2] .. ':free'
		local bid = redis.call('SPOP', free_key)
		if bid then
			local busy_key = ARGV[1] .. worker .. ':' .. ARGV[2] .. ':busy'
			redis.call('SADD', busy_key, bid)
			return {worker, bid}
		end
	end
	return nil
	`

	// 3. Execute Lua Script
	cmd := c.R.Eval(c.ctx, luaScript, []string{WorkersSet}, RedisPrefix, browserType)
	result, err := cmd.Result()
	if err != nil {
		return nil, NewBrowserError("Redis Lua Error: %v", err)
	}

	if result == nil {
		return nil, NewBrowserError("No browsers available for type: '%s'. Check workers.", browserType)
	}

	// 4. Parse Result ([worker_name, browser_id])
	resSlice, ok := result.([]interface{})
	if !ok || len(resSlice) < 2 {
		return nil, NewBrowserError("Invalid Lua response format")
	}
	workerName := resSlice[0].(string)
	bid := resSlice[1].(string)

	// 5. Initialize Session
	c.Session = &Session{
		BrowserID:   bid,
		WorkerName:  workerName,
		BrowserType: browserType,
		Video:       video,
		Record:      record,
		ProfileID:   profileID,
	}

	// If persistence/video/record is needed, we must ensure the worker is ready.
	// In Python, you called get_title to force initialization.
	if profileID != "" || video || record {
		fmt.Printf("[SDK] Initializing persistent environment on %s...\n", workerName)
		_, _ = c.Send("get_title", nil)
	}

	return map[string]interface{}{
		"status":     "ok",
		"browser_id": bid,
		"worker":     workerName,
	}, nil
}

// Release cleanly closes the session, stopping video/recordings if active.
func (c *Client) Release() (map[string]interface{}, error) {
	if c.Session == nil {
		return map[string]interface{}{"status": "error", "error": "not_acquired"}, nil
	}

	defer func() {
		c.Session = nil
	}()

	// 1. Stop Video if active
	if c.Session.Video {
		fmt.Println("[SDK] Stopping video...")
		// Use a longer timeout for video processing (120s)
		res, err := c.SendWithTimeout("stop_video", nil, 120*time.Second)
		if err == nil {
			if url, ok := res["video_url"].(string); ok {
				c.VideoURL = url
				fmt.Printf("[SDK] Session Video: %s\n", c.VideoURL)
			}
		}
	}

	// 2. Stop Record (RRWeb) if active
	if c.Session.Record {
		fmt.Println("[SDK] Finalizing session record (RRWeb)...")
		res, err := c.SendWithTimeout("stop_record", nil, 60*time.Second)
		if err == nil {
			if url, ok := res["record_url"].(string); ok {
				c.RecordURL = url
				fmt.Printf("[SDK] Session Record URL: %s\n", c.RecordURL)
			}
		}
	}

	// 3. Release Browser
	fmt.Println("[SDK] Sending release command...")
	res, err := c.Send("release_browser", nil)
	if err != nil {
		fmt.Printf("[SDK ERROR] Error inside release: %v\n", err)
		return map[string]interface{}{"status": "error", "error": err.Error()}, err
	}

	c.SessionData = res
	return res, nil
}
