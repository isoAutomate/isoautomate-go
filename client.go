package isoautomate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type BrowserClient struct {
	rdb         *redis.Client
	ctx         context.Context
	Session     *Session
	VideoURL    string
	SessionData BrowserResponse
}

// NewClient initializes the SDK.
func NewClient(redisURL string) *BrowserClient {
	_ = godotenv.Load() // Load .env

	ctx := context.Background()
	var rdb *redis.Client

	if redisURL != "" {
		opt, _ := redis.ParseURL(redisURL)
		rdb = redis.NewClient(opt)
	} else {
		host := getEnv("REDIS_HOST", DefaultRedisHost)
		port := getEnv("REDIS_PORT", DefaultRedisPort)
		pass := getEnv("REDIS_PASSWORD", "")
		dbStr := getEnv("REDIS_DB", DefaultRedisDB)
		db, _ := strconv.Atoi(dbStr)
		
		rdb = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", host, port),
			Password: pass,
			DB:       db,
		})
	}

	return &BrowserClient{rdb: rdb, ctx: ctx}
}

func (c *BrowserClient) Close() {
	if c.Session != nil {
		fmt.Println("[SDK] Auto-releasing session...")
		c.Release()
	}
	c.rdb.Close()
}

// ---------------------------- Lifecycle ----------------------------

func (c *BrowserClient) Acquire(browserType string, record bool) error {
	workers, err := c.rdb.SMembers(c.ctx, WorkersSet).Result()
	if err != nil || len(workers) == 0 {
		return errors.New("no workers found in isoFleet")
	}

	rand.Shuffle(len(workers), func(i, j int) { workers[i], workers[j] = workers[j], workers[i] })

	for _, worker := range workers {
		freeKey := fmt.Sprintf("%s%s:%s:free", RedisPrefix, worker, browserType)
		bid, err := c.rdb.SPop(c.ctx, freeKey).Result()
		if err == redis.Nil { continue } else if err != nil { continue }

		c.rdb.SAdd(c.ctx, fmt.Sprintf("%s%s:%s:busy", RedisPrefix, worker, browserType), bid)

		c.Session = &Session{
			BrowserID:   bid,
			WorkerName:  worker,
			BrowserType: browserType,
			Record:      record,
		}

		if record { c.Send("start_recording", nil, 5) }
		return nil
	}
	return fmt.Errorf("no available browsers for type: %s", browserType)
}

func (c *BrowserClient) Release() (BrowserResponse, error) {
	if c.Session == nil { return nil, errors.New("no active session") }

	if c.Session.Record {
		fmt.Println("[SDK] Stopping recording...")
		res, _ := c.Send("stop_recording", nil, 120)
		if v, ok := res["video_url"].(string); ok {
			c.VideoURL = v
			fmt.Printf("[SDK] Session Video: %s\n", c.VideoURL)
		}
	}

	fmt.Println("[SDK] Sending release command...")
	res, err := c.Send("release_browser", nil, 60)
	c.SessionData = res
	c.Session = nil
	return res, err
}

// ---------------------------- Core Communication ----------------------------

func (c *BrowserClient) Send(action string, args map[string]interface{}, timeoutSeconds int) (BrowserResponse, error) {
	if c.Session == nil { return nil, errors.New("session not acquired") }
	if args == nil { args = make(map[string]interface{}) }

	taskID := uuid.New().String()
	// Strip hyphens to match typical Python UUID hex, though not strictly required if workers handle standard UUIDs
	taskIDHex := strings.ReplaceAll(taskID, "-", "")

	resultKey := fmt.Sprintf("%sresult:%s", RedisPrefix, taskIDHex)
	queue := fmt.Sprintf("%s%s:tasks", RedisPrefix, c.Session.WorkerName)

	payload := CommandPayload{
		TaskID:      taskIDHex,
		BrowserID:   c.Session.BrowserID,
		WorkerName:  c.Session.WorkerName,
		BrowserType: c.Session.BrowserType,
		Action:      action,
		Args:        args,
		ResultKey:   resultKey,
	}

	jsonBytes, _ := json.Marshal(payload)
	c.rdb.RPush(c.ctx, queue, string(jsonBytes))

	start := time.Now()
	timeout := time.Duration(timeoutSeconds) * time.Second

	for time.Since(start) < timeout {
		val, err := c.rdb.Get(c.ctx, resultKey).Result()
		if err == redis.Nil {
			time.Sleep(50 * time.Millisecond)
			continue
		} else if err != nil { return nil, err }

		c.rdb.Del(c.ctx, resultKey)
		var res BrowserResponse
		if err := json.Unmarshal([]byte(val), &res); err != nil { return nil, err }
		return res, nil
	}
	return nil, errors.New("timeout waiting for worker response")
}

// ---------------------------- Assertion Handler ----------------------------

func (c *BrowserClient) handleAssertion(action string, args map[string]interface{}) error {
	args["screenshot"] = true
	res, _ := c.Send(action, args, 45)

	status, _ := res["status"].(string)
	if status == "fail" {
		// 1. Save Screenshot
		if b64, ok := res["screenshot_base64"].(string); ok {
			selector := "unknown"
			if s, ok := args["selector"].(string); ok { selector = s }
			// Clean selector for filename
			selectorClean := strings.NewReplacer("#", "", ".", "", " ", "_").Replace(selector)
			if len(selectorClean) > 20 { selectorClean = selectorClean[:20] }
			
			timestamp := time.Now().Format("150405")
			fname := fmt.Sprintf("FAIL_%s_%s_%s.png", action, selectorClean, timestamp)
			path := filepath.Join(AssertionFolder, fname)
			
			if err := saveFileDecoded(path, b64); err == nil {
				fmt.Printf(" [Assertion Fail] Screenshot saved: %s\n", path)
			}
		}
		// 2. Return Error
		if errMsg, ok := res["error"].(string); ok {
			return errors.New(errMsg)
		}
		return errors.New("assertion failed")
	}
	return nil
}

// =========================================================================
//  ACTION METHODS
// =========================================================================

// --- 1. Navigation & Setup ---

func (c *BrowserClient) OpenURL(url string) (BrowserResponse, error) {
	return c.Send("open_url", map[string]interface{}{"url": url}, 60)
}

func (c *BrowserClient) Reload(ignoreCache bool, script string) (BrowserResponse, error) {
	return c.Send("reload", map[string]interface{}{"ignore_cache": ignoreCache, "script_to_evaluate_on_load": script}, 60)
}

func (c *BrowserClient) Refresh() (BrowserResponse, error) { return c.Send("refresh", nil, 60) }
func (c *BrowserClient) GoBack() (BrowserResponse, error) { return c.Send("go_back", nil, 60) }
func (c *BrowserClient) GoForward() (BrowserResponse, error) { return c.Send("go_forward", nil, 60) }

func (c *BrowserClient) InternalizeLinks() (BrowserResponse, error) {
	return c.Send("internalize_links", nil, 60)
}

func (c *BrowserClient) GetNavigationHistory() (BrowserResponse, error) {
	return c.Send("get_navigation_history", nil, 60)
}

// --- 2. Mouse Interaction ---

func (c *BrowserClient) Click(selector string, timeout float64) (BrowserResponse, error) {
	return c.Send("click", map[string]interface{}{"selector": selector, "timeout": timeout}, 60)
}

func (c *BrowserClient) ClickIfVisible(selector string) (BrowserResponse, error) {
	return c.Send("click_if_visible", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) ClickVisibleElements(selector string, limit int) (BrowserResponse, error) {
	return c.Send("click_visible_elements", map[string]interface{}{"selector": selector, "limit": limit}, 60)
}

func (c *BrowserClient) ClickNthElement(selector string, number int) (BrowserResponse, error) {
	return c.Send("click_nth_element", map[string]interface{}{"selector": selector, "number": number}, 60)
}

func (c *BrowserClient) ClickNthVisibleElement(selector string, number int) (BrowserResponse, error) {
	return c.Send("click_nth_visible_element", map[string]interface{}{"selector": selector, "number": number}, 60)
}

func (c *BrowserClient) ClickLink(text string) (BrowserResponse, error) {
	return c.Send("click_link", map[string]interface{}{"text": text}, 60)
}

func (c *BrowserClient) ClickActiveElement() (BrowserResponse, error) {
	return c.Send("click_active_element", nil, 60)
}

func (c *BrowserClient) MouseClick(selector string) (BrowserResponse, error) {
	return c.Send("mouse_click", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) NestedClick(parentSelector, selector string) (BrowserResponse, error) {
	return c.Send("nested_click", map[string]interface{}{"parent_selector": parentSelector, "selector": selector}, 60)
}

func (c *BrowserClient) ClickWithOffset(selector string, x, y int, center bool) (BrowserResponse, error) {
	return c.Send("click_with_offset", map[string]interface{}{"selector": selector, "x": x, "y": y, "center": center}, 60)
}

// --- 3. Keyboard & Input ---

func (c *BrowserClient) Type(selector, text string, timeout float64) (BrowserResponse, error) {
	return c.Send("type", map[string]interface{}{"selector": selector, "text": text, "timeout": timeout}, 60)
}

func (c *BrowserClient) PressKeys(selector, text string) (BrowserResponse, error) {
	return c.Send("press_keys", map[string]interface{}{"selector": selector, "text": text}, 60)
}

func (c *BrowserClient) SendKeys(selector, text string) (BrowserResponse, error) {
	return c.Send("send_keys", map[string]interface{}{"selector": selector, "text": text}, 60)
}

func (c *BrowserClient) SetValue(selector, text string) (BrowserResponse, error) {
	return c.Send("set_value", map[string]interface{}{"selector": selector, "text": text}, 60)
}

func (c *BrowserClient) Clear(selector string) (BrowserResponse, error) {
	return c.Send("clear", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) ClearInput(selector string) (BrowserResponse, error) {
	return c.Send("clear_input", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) Submit(selector string) (BrowserResponse, error) {
	return c.Send("submit", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) Focus(selector string) (BrowserResponse, error) {
	return c.Send("focus", map[string]interface{}{"selector": selector}, 60)
}

// --- 4. GUI / Profiled (PyAutoGUI) ---

func (c *BrowserClient) GUIClickElement(selector string, timeframe float64) (BrowserResponse, error) {
	return c.Send("gui_click_element", map[string]interface{}{"selector": selector, "timeframe": timeframe}, 60)
}

func (c *BrowserClient) GUIClickXY(x, y int, timeframe float64) (BrowserResponse, error) {
	return c.Send("gui_click_x_y", map[string]interface{}{"x": x, "y": y, "timeframe": timeframe}, 60)
}

func (c *BrowserClient) GUIClickCaptcha() (BrowserResponse, error) {
	return c.Send("gui_click_captcha", nil, 60)
}

func (c *BrowserClient) SolveCaptcha() (BrowserResponse, error) {
	return c.Send("solve_captcha", nil, 180)
}

func (c *BrowserClient) GUIDragAndDrop(dragSelector, dropSelector string, timeframe float64) (BrowserResponse, error) {
	return c.Send("gui_drag_and_drop", map[string]interface{}{"drag_selector": dragSelector, "drop_selector": dropSelector, "timeframe": timeframe}, 60)
}

func (c *BrowserClient) GUIHoverElement(selector string) (BrowserResponse, error) {
	return c.Send("gui_hover_element", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) GUIWrite(text string) (BrowserResponse, error) {
	return c.Send("gui_write", map[string]interface{}{"text": text}, 60)
}

func (c *BrowserClient) GUIPressKeys(keys []string) (BrowserResponse, error) {
	return c.Send("gui_press_keys", map[string]interface{}{"keys": keys}, 60)
}

// --- 5. Selects & Dropdowns ---

func (c *BrowserClient) SelectOptionByText(selector, text string) (BrowserResponse, error) {
	return c.Send("select_option_by_text", map[string]interface{}{"selector": selector, "text": text}, 60)
}

func (c *BrowserClient) SelectOptionByValue(selector, value string) (BrowserResponse, error) {
	return c.Send("select_option_by_value", map[string]interface{}{"selector": selector, "value": value}, 60)
}

func (c *BrowserClient) SelectOptionByIndex(selector string, index int) (BrowserResponse, error) {
	return c.Send("select_option_by_index", map[string]interface{}{"selector": selector, "index": index}, 60)
}

// --- 6. Window & Tab Management ---

func (c *BrowserClient) OpenNewTab(url string) (BrowserResponse, error) {
	return c.Send("open_new_tab", map[string]interface{}{"url": url}, 60)
}

func (c *BrowserClient) OpenNewWindow(url string) (BrowserResponse, error) {
	return c.Send("open_new_window", map[string]interface{}{"url": url}, 60)
}

func (c *BrowserClient) SwitchToTab(index int) (BrowserResponse, error) {
	return c.Send("switch_to_tab", map[string]interface{}{"index": index}, 60)
}

func (c *BrowserClient) SwitchToWindow(index int) (BrowserResponse, error) {
	return c.Send("switch_to_window", map[string]interface{}{"index": index}, 60)
}

func (c *BrowserClient) CloseActiveTab() (BrowserResponse, error) { return c.Send("close_active_tab", nil, 60) }
func (c *BrowserClient) Maximize() (BrowserResponse, error) { return c.Send("maximize", nil, 60) }
func (c *BrowserClient) Minimize() (BrowserResponse, error) { return c.Send("minimize", nil, 60) }
func (c *BrowserClient) Medimize() (BrowserResponse, error) { return c.Send("medimize", nil, 60) }
func (c *BrowserClient) TileWindows() (BrowserResponse, error) { return c.Send("tile_windows", nil, 60) }

// --- 7. Data Extraction (Getters) ---

func (c *BrowserClient) GetText(selector string) (string, error) {
	if selector == "" { selector = "body" }
	res, err := c.Send("get_text", map[string]interface{}{"selector": selector}, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) GetTitle() (string, error) {
	res, err := c.Send("get_title", nil, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) GetCurrentURL() (string, error) {
	res, err := c.Send("get_current_url", nil, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) GetPageSource() (string, error) {
	// This returns the text, not a file save
	res, err := c.Send("get_page_source", nil, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) GetHTML(selector string) (string, error) {
	res, err := c.Send("get_html", map[string]interface{}{"selector": selector}, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) GetAttribute(selector, attribute string) (string, error) {
	res, err := c.Send("get_attribute", map[string]interface{}{"selector": selector, "attribute": attribute}, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) GetElementAttributes(selector string) (BrowserResponse, error) {
	return c.Send("get_element_attributes", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) GetUserAgent() (string, error) {
	res, err := c.Send("get_user_agent", nil, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) GetCookieString() (string, error) {
	res, err := c.Send("get_cookie_string", nil, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) GetElementRect(selector string) (BrowserResponse, error) {
	return c.Send("get_element_rect", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) GetWindowRect() (BrowserResponse, error) { return c.Send("get_window_rect", nil, 60) }
func (c *BrowserClient) GetScreenRect() (BrowserResponse, error) { return c.Send("get_screen_rect", nil, 60) }

func (c *BrowserClient) IsElementVisible(selector string) (bool, error) {
	res, err := c.Send("is_element_visible", map[string]interface{}{"selector": selector}, 60)
	if v, ok := res["value"].(bool); ok { return v, nil }
	return false, err
}

func (c *BrowserClient) IsTextVisible(text string) (bool, error) {
	res, err := c.Send("is_text_visible", map[string]interface{}{"text": text}, 60)
	if v, ok := res["value"].(bool); ok { return v, nil }
	return false, err
}

func (c *BrowserClient) IsChecked(selector string) (bool, error) {
	res, err := c.Send("is_checked", map[string]interface{}{"selector": selector}, 60)
	if v, ok := res["value"].(bool); ok { return v, nil }
	return false, err
}

func (c *BrowserClient) IsSelected(selector string) (bool, error) {
	res, err := c.Send("is_selected", map[string]interface{}{"selector": selector}, 60)
	if v, ok := res["value"].(bool); ok { return v, nil }
	return false, err
}

func (c *BrowserClient) IsOnline() (bool, error) {
	res, err := c.Send("is_online", nil, 60)
	if v, ok := res["value"].(bool); ok { return v, nil }
	return false, err
}

func (c *BrowserClient) GetPerformanceMetrics() (BrowserResponse, error) {
	return c.Send("get_performance_metrics", nil, 60)
}

// --- 8. Cookies & Storage ---

func (c *BrowserClient) GetAllCookies() (BrowserResponse, error) { return c.Send("get_all_cookies", nil, 60) }

func (c *BrowserClient) SaveCookies(name string) (string, error) {
	if name == "" { name = "cookies.json" }
	res, _ := c.Send("save_cookies", nil, 60)
	if v, ok := res["cookies"]; ok {
		data, _ := json.MarshalIndent(v, "", "    ")
		if err := os.WriteFile(name, data, 0644); err != nil { return "", err }
		path, _ := filepath.Abs(name)
		return path, nil
	}
	return "", errors.New("failed to retrieve cookies")
}

func (c *BrowserClient) LoadCookies(name string, cookiesList []interface{}) (BrowserResponse, error) {
	var finalCookies = cookiesList
	if finalCookies == nil && name != "" {
		if _, err := os.Stat(name); err == nil {
			data, _ := os.ReadFile(name)
			json.Unmarshal(data, &finalCookies)
		} else {
			return nil, fmt.Errorf("local cookie file not found: %s", name)
		}
	}
	return c.Send("load_cookies", map[string]interface{}{"name": name, "cookies": finalCookies}, 60)
}

func (c *BrowserClient) ClearCookies() (BrowserResponse, error) { return c.Send("clear_cookies", nil, 60) }

func (c *BrowserClient) GetLocalStorageItem(key string) (string, error) {
	res, err := c.Send("get_local_storage_item", map[string]interface{}{"key": key}, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) SetLocalStorageItem(key, value string) (BrowserResponse, error) {
	return c.Send("set_local_storage_item", map[string]interface{}{"key": key, "value": value}, 60)
}

func (c *BrowserClient) GetSessionStorageItem(key string) (string, error) {
	res, err := c.Send("get_session_storage_item", map[string]interface{}{"key": key}, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) SetSessionStorageItem(key, value string) (BrowserResponse, error) {
	return c.Send("set_session_storage_item", map[string]interface{}{"key": key, "value": value}, 60)
}

func (c *BrowserClient) ExportSession() (BrowserResponse, error) { return c.Send("get_storage_state", nil, 60) }
func (c *BrowserClient) ImportSession(state map[string]interface{}) (BrowserResponse, error) {
	return c.Send("set_storage_state", map[string]interface{}{"state": state}, 60)
}

// --- 9. Visuals & Highlights ---

func (c *BrowserClient) Highlight(selector string) (BrowserResponse, error) {
	return c.Send("highlight", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) HighlightOverlay(selector string) (BrowserResponse, error) {
	return c.Send("highlight_overlay", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) RemoveElement(selector string) (BrowserResponse, error) {
	return c.Send("remove_element", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) Flash(selector string, duration float64) (BrowserResponse, error) {
	return c.Send("flash", map[string]interface{}{"selector": selector, "duration": duration}, 60)
}

// --- 10. Advanced (MFA, Permissions, Scripting) ---

func (c *BrowserClient) GetMFACode(totpKey string) (string, error) {
	res, err := c.Send("get_mfa_code", map[string]interface{}{"totp_key": totpKey}, 30)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) EnterMFACode(selector, totpKey string) (BrowserResponse, error) {
	return c.Send("enter_mfa_code", map[string]interface{}{"selector": selector, "totp_key": totpKey}, 60)
}

func (c *BrowserClient) GrantPermissions(permissions string) (BrowserResponse, error) {
	return c.Send("grant_permissions", map[string]interface{}{"permissions": permissions}, 60)
}

func (c *BrowserClient) ExecuteScript(script string) (BrowserResponse, error) {
	return c.Send("execute_script", map[string]interface{}{"script": script}, 60)
}

func (c *BrowserClient) Evaluate(expression string) (string, error) {
	res, err := c.Send("evaluate", map[string]interface{}{"expression": expression}, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

func (c *BrowserClient) BlockURLs(patterns []string) (BrowserResponse, error) {
	return c.Send("block_urls", map[string]interface{}{"patterns": patterns}, 60)
}

// --- 11. Assertions (Commercial Grade with Screenshots) ---

func (c *BrowserClient) AssertText(text, selector string) error {
	if selector == "" { selector = "html" }
	return c.handleAssertion("assert_text", map[string]interface{}{"text": text, "selector": selector})
}

func (c *BrowserClient) AssertExactText(text, selector string) error {
	if selector == "" { selector = "html" }
	return c.handleAssertion("assert_exact_text", map[string]interface{}{"text": text, "selector": selector})
}

func (c *BrowserClient) AssertElement(selector string) error {
	return c.handleAssertion("assert_element", map[string]interface{}{"selector": selector})
}

func (c *BrowserClient) AssertElementPresent(selector string) error {
	return c.handleAssertion("assert_element_present", map[string]interface{}{"selector": selector})
}

func (c *BrowserClient) AssertElementAbsent(selector string) error {
	return c.handleAssertion("assert_element_absent", map[string]interface{}{"selector": selector})
}

func (c *BrowserClient) AssertElementNotVisible(selector string) error {
	return c.handleAssertion("assert_element_not_visible", map[string]interface{}{"selector": selector})
}

func (c *BrowserClient) AssertTextNotVisible(text, selector string) error {
	if selector == "" { selector = "html" }
	return c.handleAssertion("assert_text_not_visible", map[string]interface{}{"text": text, "selector": selector})
}

func (c *BrowserClient) AssertTitle(title string) error {
	return c.handleAssertion("assert_title", map[string]interface{}{"title": title})
}

func (c *BrowserClient) AssertURL(urlSubstring string) error {
	return c.handleAssertion("assert_url", map[string]interface{}{"url": urlSubstring})
}

func (c *BrowserClient) AssertAttribute(selector, attribute, value string) error {
	return c.handleAssertion("assert_attribute", map[string]interface{}{"selector": selector, "attribute": attribute, "value": value})
}

// --- 12. Scrolling & Waiting ---

func (c *BrowserClient) ScrollIntoView(selector string) (BrowserResponse, error) {
	return c.Send("scroll_into_view", map[string]interface{}{"selector": selector}, 60)
}

func (c *BrowserClient) ScrollToBottom() (BrowserResponse, error) { return c.Send("scroll_to_bottom", nil, 60) }
func (c *BrowserClient) ScrollToTop() (BrowserResponse, error) { return c.Send("scroll_to_top", nil, 60) }

func (c *BrowserClient) ScrollDown(amount int) (BrowserResponse, error) {
	return c.Send("scroll_down", map[string]interface{}{"amount": amount}, 60)
}
func (c *BrowserClient) ScrollUp(amount int) (BrowserResponse, error) {
	return c.Send("scroll_up", map[string]interface{}{"amount": amount}, 60)
}
func (c *BrowserClient) ScrollToY(y int) (BrowserResponse, error) {
	return c.Send("scroll_to_y", map[string]interface{}{"y": y}, 60)
}

func (c *BrowserClient) Sleep(seconds float64) (BrowserResponse, error) {
	return c.Send("sleep", map[string]interface{}{"seconds": seconds}, 60)
}

func (c *BrowserClient) WaitForElement(selector string, timeout float64) (BrowserResponse, error) {
	return c.Send("wait_for_element", map[string]interface{}{"selector": selector, "timeout": timeout}, 60)
}

func (c *BrowserClient) WaitForText(text, selector string, timeout float64) (BrowserResponse, error) {
	if selector == "" { selector = "html" }
	return c.Send("wait_for_text", map[string]interface{}{"text": text, "selector": selector, "timeout": timeout}, 60)
}

func (c *BrowserClient) WaitForElementPresent(selector string, timeout float64) (BrowserResponse, error) {
	return c.Send("wait_for_element_present", map[string]interface{}{"selector": selector, "timeout": timeout}, 60)
}

func (c *BrowserClient) WaitForElementAbsent(selector string, timeout float64) (BrowserResponse, error) {
	return c.Send("wait_for_element_absent", map[string]interface{}{"selector": selector, "timeout": timeout}, 60)
}

func (c *BrowserClient) WaitForNetworkIdle() (BrowserResponse, error) { return c.Send("wait_for_network_idle", nil, 60) }

func (c *BrowserClient) WaitForElementNotVisible(selector string, timeout float64) (BrowserResponse, error) {
	return c.Send("wait_for_element_not_visible", map[string]interface{}{"selector": selector, "timeout": timeout}, 60)
}

// --- 13. Screenshots & Files ---

func (c *BrowserClient) SavePageSource(name string) (string, error) {
	if name == "" { name = "source.html" }
	res, _ := c.Send("save_page_source", nil, 60)
	if b64, ok := res["source_base64"].(string); ok {
		data, _ := base64.StdEncoding.DecodeString(b64)
		if err := os.WriteFile(name, data, 0644); err == nil {
			path, _ := filepath.Abs(name)
			return path, nil
		}
	}
	return "", errors.New("failed to save page source")
}

func (c *BrowserClient) Screenshot(filename, selector string) (string, error) {
	if filename == "" {
		timestamp := time.Now().Format("20060102_150405")
		filename = filepath.Join(ScreenshotFolder, fmt.Sprintf("%s_%s.png", timestamp, uuid.New().String()[:4]))
	}
	res, err := c.Send("save_screenshot", map[string]interface{}{"name": "temp.png", "selector": selector}, 60)
	if err != nil { return "", err }
	
	if b64, ok := res["image_base64"].(string); ok {
		if err := saveFileDecoded(filename, b64); err == nil {
			path, _ := filepath.Abs(filename)
			return path, nil
		}
	}
	return "", errors.New("failed to save screenshot")
}

func (c *BrowserClient) SaveAsPDF(filename string) (string, error) {
	if filename == "" { filename = fmt.Sprintf("doc_%d.pdf", time.Now().Unix()) }
	res, err := c.Send("save_as_pdf", nil, 60)
	if err != nil { return "", err }
	
	if b64, ok := res["pdf_base64"].(string); ok {
		if err := saveFileDecoded(filename, b64); err == nil {
			path, _ := filepath.Abs(filename)
			return path, nil
		}
	}
	return "", errors.New("failed to save PDF")
}

func (c *BrowserClient) UploadFile(selector, filePath string) (BrowserResponse, error) {
	return c.Send("upload_file", map[string]interface{}{"selector": selector, "file_path": filePath}, 60)
}

// --- New Mouse Actions ---

func (c *BrowserClient) DoubleClick(selector string) (BrowserResponse, error) {
	return c.Send("double_click", map[string]interface{}{"selector": selector}, 60)
}
func (c *BrowserClient) RightClick(selector string) (BrowserResponse, error) {
	return c.Send("right_click", map[string]interface{}{"selector": selector}, 60)
}
func (c *BrowserClient) Hover(selector string) (BrowserResponse, error) {
	return c.Send("hover", map[string]interface{}{"selector": selector}, 60)
}
func (c *BrowserClient) DragAndDrop(dragSelector, dropSelector string) (BrowserResponse, error) {
	return c.Send("drag_and_drop", map[string]interface{}{"drag_selector": dragSelector, "drop_selector": dropSelector}, 60)
}

// --- New Frame Actions ---

func (c *BrowserClient) SwitchToFrame(selector string) (BrowserResponse, error) {
	return c.Send("switch_to_frame", map[string]interface{}{"selector": selector}, 60)
}
func (c *BrowserClient) SwitchToDefaultContent() (BrowserResponse, error) { return c.Send("switch_to_default_content", nil, 60) }
func (c *BrowserClient) SwitchToParentFrame() (BrowserResponse, error) { return c.Send("switch_to_parent_frame", nil, 60) }

// --- New Alert Actions ---

func (c *BrowserClient) AcceptAlert() (BrowserResponse, error) { return c.Send("accept_alert", nil, 60) }
func (c *BrowserClient) DismissAlert() (BrowserResponse, error) { return c.Send("dismiss_alert", nil, 60) }
func (c *BrowserClient) GetAlertText() (string, error) {
	res, err := c.Send("get_alert_text", nil, 60)
	if v, ok := res["value"].(string); ok { return v, nil }
	return "", err
}

// --- New Granular Cookie Actions ---

func (c *BrowserClient) AddCookie(cookie map[string]interface{}) (BrowserResponse, error) {
	return c.Send("add_cookie", map[string]interface{}{"cookie": cookie}, 60)
}
func (c *BrowserClient) DeleteCookie(name string) (BrowserResponse, error) {
	return c.Send("delete_cookie", map[string]interface{}{"name": name}, 60)
}

// --- New Viewport Action ---

func (c *BrowserClient) SetWindowSize(width, height int) (BrowserResponse, error) {
	return c.Send("set_window_size", map[string]interface{}{"width": width, "height": height}, 60)
}
func (c *BrowserClient) SetWindowRect(x, y, width, height int) (BrowserResponse, error) {
	return c.Send("set_window_rect", map[string]interface{}{"x": x, "y": y, "width": width, "height": height}, 60)
}