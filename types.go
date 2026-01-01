package isoautomate

// Session represents the active browser session details
type Session struct {
	BrowserID   string `json:"browser_id"`
	WorkerName  string `json:"worker"`
	BrowserType string `json:"browser_type"`
	Record      bool   `json:"record"`
}

// CommandPayload is the exact JSON structure the Python Engine expects
type CommandPayload struct {
	TaskID      string                 `json:"task_id"`
	BrowserID   string                 `json:"browser_id"`
	WorkerName  string                 `json:"worker_name"`
	BrowserType string                 `json:"browser_type"`
	Action      string                 `json:"action"`
	Args        map[string]interface{} `json:"args"`
	ResultKey   string                 `json:"result_key"`
}

// BrowserResponse is a generic map to handle dynamic JSON responses
type BrowserResponse map[string]interface{}