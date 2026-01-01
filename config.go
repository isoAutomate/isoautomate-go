package isoautomate

import (
	"os"
	"path/filepath"
)

// Redis Protocol Constants
const (
	RedisPrefix      = "ISOAUTOMATE:"
	WorkersSet       = RedisPrefix + "workers"
	DefaultRedisHost = "localhost"
	DefaultRedisPort = "6379"
	DefaultRedisDB   = "0"
)

// File Paths (defaults)
var (
	ScreenshotFolder = "screenshots"
	AssertionFolder  = filepath.Join(ScreenshotFolder, "failures")
)

// getEnv is a helper to read env vars with a default fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}