package isoautomate

import "os"

// Constants defining the Protocol
const (
	RedisPrefix      = "ISOAUTOMATE:"
	WorkersSet       = RedisPrefix + "workers"
	ScreenshotFolder = "screenshots"
)

// AssertionFolder is determined at runtime
var AssertionFolder = ScreenshotFolder + string(os.PathSeparator) + "failures"

// Config holds the connection details
type Config struct {
	RedisURL      string
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int
	RedisSSL      bool
	EnvFile       string // Custom path to .env file
}
