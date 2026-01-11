package isoautomate

// Session represents the active browser session
type Session struct {
	BrowserID   string `json:"browser_id"`
	WorkerName  string `json:"worker"`
	BrowserType string `json:"browser_type"`
	Video       bool   `json:"video"`
	Record      bool   `json:"record"`
	ProfileID   string `json:"profile_id,omitempty"`
}

// TaskPayload represents the JSON sent TO Redis (RPUSH)
type TaskPayload struct {
	TaskID      string                 `json:"task_id"`
	BrowserID   string                 `json:"browser_id"`
	WorkerName  string                 `json:"worker_name"`
	Action      string                 `json:"action"`
	Args        map[string]interface{} `json:"args"`
	ResultKey   string                 `json:"result_key"`
	Video       bool                   `json:"video,omitempty"`
	Record      bool                   `json:"record,omitempty"`
	ProfileID   string                 `json:"profile_id,omitempty"`
	BrowserType string                 `json:"browser_type,omitempty"`
}

// TaskResponse represents the JSON received FROM Redis (BLPOP)
type TaskResponse struct {
	Status           string      `json:"status"`
	Error            string      `json:"error,omitempty"`
	VideoURL         string      `json:"video_url,omitempty"`
	RecordURL        string      `json:"record_url,omitempty"`
	ScreenshotBase64 string      `json:"screenshot_base64,omitempty"`
	Data             interface{} `json:"data,omitempty"`
}
