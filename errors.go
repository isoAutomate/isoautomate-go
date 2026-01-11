package isoautomate

import "fmt"

// BrowserError is the custom error type for the SDK
type BrowserError struct {
	Message string
}

func (e *BrowserError) Error() string {
	return fmt.Sprintf("isoAutomate Error: %s", e.Message)
}

// NewBrowserError helps create a new error
func NewBrowserError(format string, a ...interface{}) error {
	return &BrowserError{
		Message: fmt.Sprintf(format, a...),
	}
}
