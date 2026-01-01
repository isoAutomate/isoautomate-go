package isoautomate

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

// saveFileDecoded writes a Base64 string to a local file
func saveFileDecoded(pathStr, base64Data string) error {
	// Ensure directory exists
	dir := filepath.Dir(pathStr)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode base64: %v", err)
	}

	return os.WriteFile(pathStr, data, 0644)
}