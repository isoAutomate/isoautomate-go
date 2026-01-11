package isoautomate

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// LoadEnv loads .env variables from a custom path or standard locations.
func LoadEnv(customPath string) {
	// 1. Try custom path if provided
	if customPath != "" {
		_ = godotenv.Overload(customPath)
		return
	}

	// 2. Try Current Working Directory
	cwd, _ := os.Getwd()
	cwdEnv := filepath.Join(cwd, ".env")
	_ = godotenv.Load(cwdEnv)
}

// SaveBase64File decodes a base64 string and writes it to the output path.
// It ensures the directory exists.
func SaveBase64File(base64Data, outputPath string) (string, error) {
	// Decode
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", NewBrowserError("Failed to decode base64 data: %v", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", NewBrowserError("Failed to create directory: %v", err)
	}

	// Write file
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return "", NewBrowserError("Failed to write file: %v", err)
	}

	// Return absolute path
	absPath, _ := filepath.Abs(outputPath)
	return absPath, nil
}

// cleanSelector formats selectors for filenames (removes #, ., spaces)
func cleanSelector(s string) string {
	s = strings.ReplaceAll(s, "#", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, " ", "_")
	if len(s) > 20 {
		return s[:20]
	}
	return s
}
