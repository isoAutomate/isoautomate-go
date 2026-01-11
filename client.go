package isoautomate

import (
	"context"
	"crypto/tls"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client is the main entry point for the SDK.
type Client struct {
	R           *redis.Client          // Redis Connection
	Session     *Session               // Active Session Data
	SessionData map[string]interface{} // Metadata from Release
	VideoURL    string
	RecordURL   string
	InitSent    bool // Tracks if we've sent the first command

	// Context for Redis operations
	ctx context.Context
}

// New creates a new Client instance and connects to Redis.
func New(cfg Config) (*Client, error) {
	// 1. Load Environment Variables
	LoadEnv(cfg.EnvFile)

	// 2. Resolve Config (Env vars override defaults, explicit config overrides env)
	host := cfg.RedisHost
	if host == "" {
		host = os.Getenv("REDIS_HOST")
	}

	port := cfg.RedisPort
	if port == "" {
		port = os.Getenv("REDIS_PORT")
	}
	if port == "" {
		port = "6379" // Default
	}

	password := cfg.RedisPassword
	if password == "" {
		password = os.Getenv("REDIS_PASSWORD")
	}

	db := cfg.RedisDB // Default is 0, which is fine
	if db == 0 && os.Getenv("REDIS_DB") != "" {
		d, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
		db = d
	}

	// SSL/TLS Logic
	useSSL := cfg.RedisSSL
	if !useSSL && (os.Getenv("REDIS_SSL") == "true" || os.Getenv("REDIS_SSL") == "1") {
		useSSL = true
	}

	// 3. Setup Redis Options
	var rdb *redis.Client

	// If a full URL is provided
	if cfg.RedisURL != "" || os.Getenv("REDIS_URL") != "" {
		url := cfg.RedisURL
		if url == "" {
			url = os.Getenv("REDIS_URL")
		}
		opts, err := redis.ParseURL(url)
		if err != nil {
			return nil, NewBrowserError("Invalid Redis URL: %v", err)
		}
		rdb = redis.NewClient(opts)
	} else {
		// Manual Configuration
		if host == "" {
			return nil, NewBrowserError("Missing Redis Configuration (Host)")
		}

		redisOptions := &redis.Options{
			Addr:     host + ":" + port,
			Password: password,
			DB:       db,
		}

		if useSSL {
			redisOptions.TLSConfig = &tls.Config{
				InsecureSkipVerify: true, // Common for internal Redis, adjust if needed
			}
		}
		rdb = redis.NewClient(redisOptions)
	}

	// 4. Test Connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, NewBrowserError("Failed to connect to Redis: %v", err)
	}

	return &Client{
		R:           rdb,
		ctx:         context.Background(),
		SessionData: make(map[string]interface{}),
	}, nil
}
