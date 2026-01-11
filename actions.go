package isoautomate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// --- File & Screenshot Actions ---

func (c *Client) Screenshot(filename string, selector string) (map[string]interface{}, error) {
	if filename == "" {
		timestamp := time.Now().Format("20060102_150405")
		uniqueID := uuid.New().Hex()[:4]
		filename = filepath.Join(ScreenshotFolder, fmt.Sprintf("%s_%s.png", timestamp, uniqueID))
	}

	args := map[string]interface{}{"name": "temp.png"}
	if selector != "" {
		args["selector"] = selector
	}

	res, err := c.Send("save_screenshot", args)
	if err != nil {
		return nil, err
	}
	return c.saveBase64Result(res, "image_base64", filename)
}

func (c *Client) SaveAsPDF(filename string) (map[string]interface{}, error) {
	if filename == "" {
		filename = fmt.Sprintf("doc_%d.pdf", time.Now().Unix())
	}
	res, err := c.Send("save_as_pdf", nil)
	if err != nil {
		return nil, err
	}
	return c.saveBase64Result(res, "pdf_base64", filename)
}

func (c *Client) SavePageSource(name string) (map[string]interface{}, error) {
	if name == "" {
		name = "source.html"
	}
	res, err := c.Send("save_page_source", nil)
	if err != nil {
		return nil, err
	}

	// Python logic: decode source_base64 and write text
	if status, ok := res["status"].(string); ok && status == "ok" {
		if b64, ok := res["source_base64"].(string); ok {
			data, decodeErr := base64.StdEncoding.DecodeString(b64)
			if decodeErr != nil {
				return map[string]interface{}{"status": "error", "error": decodeErr.Error()}, nil
			}
			if writeErr := os.WriteFile(name, data, 0644); writeErr != nil {
				return map[string]interface{}{"status": "error", "error": writeErr.Error()}, nil
			}
			absPath, _ := filepath.Abs(name)
			return map[string]interface{}{"status": "ok", "path": absPath}, nil
		}
	}
	return res, nil
}

func (c *Client) ExecuteCDPCmd(cmd string, params map[string]interface{}) (map[string]interface{}, error) {
	return c.Send("execute_cdp_cmd", map[string]interface{}{
		"cmd":    cmd,
		"params": params,
	})
}

func (c *Client) UploadFile(selector string, localFilePath string) (map[string]interface{}, error) {
	if _, err := os.Stat(localFilePath); os.IsNotExist(err) {
		return map[string]interface{}{"status": "error", "error": fmt.Sprintf("Local file not found: %s", localFilePath)}, nil
	}

	data, err := os.ReadFile(localFilePath)
	if err != nil {
		return nil, err
	}

	// Encode to base64
	encodedData := base64.StdEncoding.EncodeToString(data)
	filename := filepath.Base(localFilePath)

	return c.Send("upload_file", map[string]interface{}{
		"selector":  selector,
		"file_name": filename,
		"file_data": encodedData,
	})
}

// --- Navigation ---

func (c *Client) OpenURL(url string) (map[string]interface{}, error) {
	return c.Send("open_url", map[string]interface{}{"url": url})
}

func (c *Client) Reload(ignoreCache bool, script string) (map[string]interface{}, error) {
	return c.Send("reload", map[string]interface{}{
		"ignore_cache":               ignoreCache,
		"script_to_evaluate_on_load": script,
	})
}

func (c *Client) Refresh() (map[string]interface{}, error) {
	return c.Send("refresh", nil)
}

func (c *Client) GoBack() (map[string]interface{}, error) {
	return c.Send("go_back", nil)
}

func (c *Client) GoForward() (map[string]interface{}, error) {
	return c.Send("go_forward", nil)
}

func (c *Client) InternalizeLinks() (map[string]interface{}, error) {
	return c.Send("internalize_links", nil)
}

func (c *Client) GetNavigationHistory() (map[string]interface{}, error) {
	return c.Send("get_navigation_history", nil)
}

// --- Interaction (Clicks & Typing) ---

func (c *Client) Click(selector string, timeout int) (map[string]interface{}, error) {
	args := map[string]interface{}{"selector": selector}
	if timeout > 0 {
		args["timeout"] = timeout
	}
	return c.Send("click", args)
}

func (c *Client) ClickIfVisible(selector string) (map[string]interface{}, error) {
	return c.Send("click_if_visible", map[string]interface{}{"selector": selector})
}

func (c *Client) ClickVisibleElements(selector string, limit int) (map[string]interface{}, error) {
	return c.Send("click_visible_elements", map[string]interface{}{"selector": selector, "limit": limit})
}

func (c *Client) ClickNthElement(selector string, number int) (map[string]interface{}, error) {
	return c.Send("click_nth_element", map[string]interface{}{"selector": selector, "number": number})
}

func (c *Client) ClickNthVisibleElement(selector string, number int) (map[string]interface{}, error) {
	return c.Send("click_nth_visible_element", map[string]interface{}{"selector": selector, "number": number})
}

func (c *Client) ClickLink(text string) (map[string]interface{}, error) {
	return c.Send("click_link", map[string]interface{}{"text": text})
}

func (c *Client) ClickActiveElement() (map[string]interface{}, error) {
	return c.Send("click_active_element", nil)
}

func (c *Client) MouseClick(selector string) (map[string]interface{}, error) {
	return c.Send("mouse_click", map[string]interface{}{"selector": selector})
}

func (c *Client) NestedClick(parentSelector, selector string) (map[string]interface{}, error) {
	return c.Send("nested_click", map[string]interface{}{"parent_selector": parentSelector, "selector": selector})
}

func (c *Client) ClickWithOffset(selector string, x, y int, center bool) (map[string]interface{}, error) {
	return c.Send("click_with_offset", map[string]interface{}{
		"selector": selector,
		"x":        x,
		"y":        y,
		"center":   center,
	})
}

func (c *Client) Type(selector, text string, timeout int) (map[string]interface{}, error) {
	args := map[string]interface{}{"selector": selector, "text": text}
	if timeout > 0 {
		args["timeout"] = timeout
	}
	return c.Send("type", args)
}

func (c *Client) PressKeys(selector, text string) (map[string]interface{}, error) {
	return c.Send("press_keys", map[string]interface{}{"selector": selector, "text": text})
}

func (c *Client) SendKeys(selector, text string) (map[string]interface{}, error) {
	return c.Send("send_keys", map[string]interface{}{"selector": selector, "text": text})
}

func (c *Client) SetValue(selector, text string) (map[string]interface{}, error) {
	return c.Send("set_value", map[string]interface{}{"selector": selector, "text": text})
}

func (c *Client) Clear(selector string) (map[string]interface{}, error) {
	return c.Send("clear", map[string]interface{}{"selector": selector})
}

func (c *Client) ClearInput(selector string) (map[string]interface{}, error) {
	return c.Send("clear_input", map[string]interface{}{"selector": selector})
}

func (c *Client) Submit(selector string) (map[string]interface{}, error) {
	return c.Send("submit", map[string]interface{}{"selector": selector})
}

func (c *Client) Focus(selector string) (map[string]interface{}, error) {
	return c.Send("focus", map[string]interface{}{"selector": selector})
}

// --- GUI (Human-like) ---

func (c *Client) GuiClickElement(selector string, timeframe float64) (map[string]interface{}, error) {
	if timeframe == 0 {
		timeframe = 0.25
	}
	return c.Send("gui_click_element", map[string]interface{}{"selector": selector, "timeframe": timeframe})
}

func (c *Client) GuiClickXY(x, y int, timeframe float64) (map[string]interface{}, error) {
	if timeframe == 0 {
		timeframe = 0.25
	}
	return c.Send("gui_click_x_y", map[string]interface{}{"x": x, "y": y, "timeframe": timeframe})
}

func (c *Client) GuiClickCaptcha() (map[string]interface{}, error) {
	return c.Send("gui_click_captcha", nil)
}

func (c *Client) SolveCaptcha() (map[string]interface{}, error) {
	return c.Send("solve_captcha", nil)
}

func (c *Client) GuiDragAndDrop(dragSelector, dropSelector string, timeframe float64) (map[string]interface{}, error) {
	if timeframe == 0 {
		timeframe = 0.35
	}
	return c.Send("gui_drag_and_drop", map[string]interface{}{
		"drag_selector": dragSelector,
		"drop_selector": dropSelector,
		"timeframe":     timeframe,
	})
}

func (c *Client) GuiHoverElement(selector string) (map[string]interface{}, error) {
	return c.Send("gui_hover_element", map[string]interface{}{"selector": selector})
}

func (c *Client) GuiWrite(text string) (map[string]interface{}, error) {
	return c.Send("gui_write", map[string]interface{}{"text": text})
}

func (c *Client) GuiPressKeys(keys []string) (map[string]interface{}, error) {
	return c.Send("gui_press_keys", map[string]interface{}{"keys": keys})
}

// --- Select / Options ---

func (c *Client) SelectOptionByText(selector, text string) (map[string]interface{}, error) {
	return c.Send("select_option_by_text", map[string]interface{}{"selector": selector, "text": text})
}

func (c *Client) SelectOptionByValue(selector, value string) (map[string]interface{}, error) {
	return c.Send("select_option_by_value", map[string]interface{}{"selector": selector, "value": value})
}

func (c *Client) SelectOptionByIndex(selector string, index int) (map[string]interface{}, error) {
	return c.Send("select_option_by_index", map[string]interface{}{"selector": selector, "index": index})
}

// --- Windows & Tabs ---

func (c *Client) OpenNewTab(url string) (map[string]interface{}, error) {
	return c.Send("open_new_tab", map[string]interface{}{"url": url})
}

func (c *Client) OpenNewWindow(url string) (map[string]interface{}, error) {
	return c.Send("open_new_window", map[string]interface{}{"url": url})
}

func (c *Client) SwitchToTab(index int) (map[string]interface{}, error) {
	return c.Send("switch_to_tab", map[string]interface{}{"index": index})
}

func (c *Client) SwitchToWindow(index int) (map[string]interface{}, error) {
	return c.Send("switch_to_window", map[string]interface{}{"index": index})
}

func (c *Client) CloseActiveTab() (map[string]interface{}, error) {
	return c.Send("close_active_tab", nil)
}

func (c *Client) Maximize() (map[string]interface{}, error) {
	return c.Send("maximize", nil)
}

func (c *Client) Minimize() (map[string]interface{}, error) {
	return c.Send("minimize", nil)
}

func (c *Client) Medimize() (map[string]interface{}, error) {
	return c.Send("medimize", nil)
}

func (c *Client) TileWindows() (map[string]interface{}, error) {
	return c.Send("tile_windows", nil)
}

// --- Getters ---

func (c *Client) GetText(selector string) (map[string]interface{}, error) {
	if selector == "" {
		selector = "body"
	}
	return c.Send("get_text", map[string]interface{}{"selector": selector})
}

func (c *Client) GetTitle() (map[string]interface{}, error) {
	return c.Send("get_title", nil)
}

func (c *Client) GetCurrentURL() (map[string]interface{}, error) {
	return c.Send("get_current_url", nil)
}

func (c *Client) GetPageSource() (map[string]interface{}, error) {
	return c.Send("get_page_source", nil)
}

func (c *Client) GetHTML(selector string) (map[string]interface{}, error) {
	return c.Send("get_html", map[string]interface{}{"selector": selector})
}

func (c *Client) GetAttribute(selector, attribute string) (map[string]interface{}, error) {
	return c.Send("get_attribute", map[string]interface{}{"selector": selector, "attribute": attribute})
}

func (c *Client) GetElementAttributes(selector string) (map[string]interface{}, error) {
	return c.Send("get_element_attributes", map[string]interface{}{"selector": selector})
}

func (c *Client) GetUserAgent() (map[string]interface{}, error) {
	return c.Send("get_user_agent", nil)
}

func (c *Client) GetCookieString() (map[string]interface{}, error) {
	return c.Send("get_cookie_string", nil)
}

func (c *Client) GetElementRect(selector string) (map[string]interface{}, error) {
	return c.Send("get_element_rect", map[string]interface{}{"selector": selector})
}

func (c *Client) GetWindowRect() (map[string]interface{}, error) {
	return c.Send("get_window_rect", nil)
}

func (c *Client) GetScreenRect() (map[string]interface{}, error) {
	return c.Send("get_screen_rect", nil)
}

func (c *Client) IsElementVisible(selector string) (map[string]interface{}, error) {
	return c.Send("is_element_visible", map[string]interface{}{"selector": selector})
}

func (c *Client) IsTextVisible(text string) (map[string]interface{}, error) {
	return c.Send("is_text_visible", map[string]interface{}{"text": text})
}

func (c *Client) IsChecked(selector string) (map[string]interface{}, error) {
	return c.Send("is_checked", map[string]interface{}{"selector": selector})
}

func (c *Client) IsSelected(selector string) (map[string]interface{}, error) {
	return c.Send("is_selected", map[string]interface{}{"selector": selector})
}

func (c *Client) IsOnline() (map[string]interface{}, error) {
	return c.Send("is_online", nil)
}

func (c *Client) GetPerformanceMetrics() (map[string]interface{}, error) {
	return c.Send("get_performance_metrics", nil)
}

// --- Cookies & Storage ---

func (c *Client) GetAllCookies() (map[string]interface{}, error) {
	return c.Send("get_all_cookies", nil)
}

func (c *Client) SaveCookies(name string) (map[string]interface{}, error) {
	if name == "" {
		name = "cookies.txt"
	}
	res, err := c.Send("save_cookies", nil)
	if err != nil {
		return nil, err
	}

	if status, ok := res["status"].(string); ok && status == "ok" {
		if cookies, ok := res["cookies"]; ok {
			data, _ := json.MarshalIndent(cookies, "", "    ")
			if err := os.WriteFile(name, data, 0644); err != nil {
				return map[string]interface{}{"status": "error", "error": fmt.Sprintf("Failed to write local file: %v", err)}, nil
			}
			absPath, _ := filepath.Abs(name)
			return map[string]interface{}{"status": "ok", "path": absPath}, nil
		}
	}
	return res, nil
}

func (c *Client) LoadCookies(name string, cookiesList interface{}) (map[string]interface{}, error) {
	finalCookies := cookiesList

	// If no list provided, load from file
	if finalCookies == nil && name != "" {
		if _, err := os.Stat(name); os.IsNotExist(err) {
			return map[string]interface{}{"status": "error", "error": fmt.Sprintf("Local cookie file not found: %s", name)}, nil
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return map[string]interface{}{"status": "error", "error": fmt.Sprintf("Failed to read local file: %v", err)}, nil
		}

		var loaded interface{}
		if err := json.Unmarshal(data, &loaded); err != nil {
			return map[string]interface{}{"status": "error", "error": fmt.Sprintf("Failed to parse cookie file: %v", err)}, nil
		}
		finalCookies = loaded
	}

	return c.Send("load_cookies", map[string]interface{}{
		"name":    name,
		"cookies": finalCookies,
	})
}

func (c *Client) ClearCookies() (map[string]interface{}, error) {
	return c.Send("clear_cookies", nil)
}

func (c *Client) GetLocalStorageItem(key string) (map[string]interface{}, error) {
	return c.Send("get_local_storage_item", map[string]interface{}{"key": key})
}

func (c *Client) SetLocalStorageItem(key, value string) (map[string]interface{}, error) {
	return c.Send("set_local_storage_item", map[string]interface{}{"key": key, "value": value})
}

func (c *Client) GetSessionStorageItem(key string) (map[string]interface{}, error) {
	return c.Send("get_session_storage_item", map[string]interface{}{"key": key})
}

func (c *Client) SetSessionStorageItem(key, value string) (map[string]interface{}, error) {
	return c.Send("set_session_storage_item", map[string]interface{}{"key": key, "value": value})
}

func (c *Client) ExportSession() (map[string]interface{}, error) {
	return c.Send("get_storage_state", nil)
}

func (c *Client) ImportSession(stateDict map[string]interface{}) (map[string]interface{}, error) {
	return c.Send("set_storage_state", map[string]interface{}{"state": stateDict})
}

// --- Visual & Security ---

func (c *Client) Highlight(selector string) (map[string]interface{}, error) {
	return c.Send("highlight", map[string]interface{}{"selector": selector})
}

func (c *Client) HighlightOverlay(selector string) (map[string]interface{}, error) {
	return c.Send("highlight_overlay", map[string]interface{}{"selector": selector})
}

func (c *Client) RemoveElement(selector string) (map[string]interface{}, error) {
	return c.Send("remove_element", map[string]interface{}{"selector": selector})
}

func (c *Client) Flash(selector string, duration float64) (map[string]interface{}, error) {
	if duration == 0 {
		duration = 1
	}
	return c.Send("flash", map[string]interface{}{"selector": selector, "duration": duration})
}

func (c *Client) GetMFACode(totpKey string) (map[string]interface{}, error) {
	return c.Send("get_mfa_code", map[string]interface{}{"totp_key": totpKey})
}

func (c *Client) EnterMFACode(selector, totpKey string) (map[string]interface{}, error) {
	return c.Send("enter_mfa_code", map[string]interface{}{"selector": selector, "totp_key": totpKey})
}

func (c *Client) GrantPermissions(permissions []string) (map[string]interface{}, error) {
	return c.Send("grant_permissions", map[string]interface{}{"permissions": permissions})
}

func (c *Client) ExecuteScript(script string) (map[string]interface{}, error) {
	return c.Send("execute_script", map[string]interface{}{"script": script})
}

func (c *Client) Evaluate(expression string) (map[string]interface{}, error) {
	return c.Send("evaluate", map[string]interface{}{"expression": expression})
}

func (c *Client) BlockURLs(patterns []string) (map[string]interface{}, error) {
	return c.Send("block_urls", map[string]interface{}{"patterns": patterns})
}

// --- Scrolling & Waiting ---

func (c *Client) ScrollIntoView(selector string) (map[string]interface{}, error) {
	return c.Send("scroll_into_view", map[string]interface{}{"selector": selector})
}

func (c *Client) ScrollToBottom() (map[string]interface{}, error) {
	return c.Send("scroll_to_bottom", nil)
}

func (c *Client) ScrollToTop() (map[string]interface{}, error) {
	return c.Send("scroll_to_top", nil)
}

func (c *Client) ScrollDown(amount int) (map[string]interface{}, error) {
	if amount == 0 {
		amount = 25
	}
	return c.Send("scroll_down", map[string]interface{}{"amount": amount})
}

func (c *Client) ScrollUp(amount int) (map[string]interface{}, error) {
	if amount == 0 {
		amount = 25
	}
	return c.Send("scroll_up", map[string]interface{}{"amount": amount})
}

func (c *Client) ScrollToY(y int) (map[string]interface{}, error) {
	return c.Send("scroll_to_y", map[string]interface{}{"y": y})
}

func (c *Client) Sleep(seconds float64) (map[string]interface{}, error) {
	return c.Send("sleep", map[string]interface{}{"seconds": seconds})
}

func (c *Client) WaitForElement(selector string, timeout int) (map[string]interface{}, error) {
	args := map[string]interface{}{"selector": selector}
	if timeout > 0 {
		args["timeout"] = timeout
	}
	return c.Send("wait_for_element", args)
}

func (c *Client) WaitForText(text, selector string, timeout int) (map[string]interface{}, error) {
	if selector == "" {
		selector = "html"
	}
	args := map[string]interface{}{"text": text, "selector": selector}
	if timeout > 0 {
		args["timeout"] = timeout
	}
	return c.Send("wait_for_text", args)
}

func (c *Client) WaitForElementPresent(selector string, timeout int) (map[string]interface{}, error) {
	args := map[string]interface{}{"selector": selector}
	if timeout > 0 {
		args["timeout"] = timeout
	}
	return c.Send("wait_for_element_present", args)
}

func (c *Client) WaitForElementAbsent(selector string, timeout int) (map[string]interface{}, error) {
	args := map[string]interface{}{"selector": selector}
	if timeout > 0 {
		args["timeout"] = timeout
	}
	return c.Send("wait_for_element_absent", args)
}

func (c *Client) WaitForNetworkIdle() (map[string]interface{}, error) {
	return c.Send("wait_for_network_idle", nil)
}

// --- Assertions ---

func (c *Client) handleAssertion(action string, args map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := args["screenshot"]; !ok {
		args["screenshot"] = true
	}

	res, err := c.Send(action, args)
	if err != nil {
		return nil, err
	}

	if status, ok := res["status"].(string); ok && status == "fail" {
		// Handle automatic screenshot on failure
		if b64, ok := res["screenshot_base64"].(string); ok {
			_ = os.MkdirAll(AssertionFolder, 0755)

			selector := "unknown"
			if s, ok := args["selector"].(string); ok {
				selector = cleanSelector(s)
			}

			timestamp := time.Now().Format("150405")
			filename := fmt.Sprintf("FAIL_%s_%s_%s.png", action, selector, timestamp)
			path := filepath.Join(AssertionFolder, filename)

			if data, err := base64.StdEncoding.DecodeString(b64); err == nil {
				_ = os.WriteFile(path, data, 0644)
				fmt.Printf("[Assertion Fail] Screenshot saved: %s\n", path)
			}
		}

		errMsg := "Unknown assertion error"
		if e, ok := res["error"].(string); ok {
			errMsg = e
		}
		// In Go, we return an error rather than raising an exception
		return res, NewBrowserError("Assertion Failed: %s", errMsg)
	}

	return res, nil
}

func (c *Client) AssertText(text, selector string, screenshot bool) (map[string]interface{}, error) {
	if selector == "" {
		selector = "html"
	}
	return c.handleAssertion("assert_text", map[string]interface{}{
		"text": text, "selector": selector, "screenshot": screenshot,
	})
}

func (c *Client) AssertExactText(text, selector string, screenshot bool) (map[string]interface{}, error) {
	if selector == "" {
		selector = "html"
	}
	return c.handleAssertion("assert_exact_text", map[string]interface{}{
		"text": text, "selector": selector, "screenshot": screenshot,
	})
}

func (c *Client) AssertElement(selector string, screenshot bool) (map[string]interface{}, error) {
	return c.handleAssertion("assert_element", map[string]interface{}{
		"selector": selector, "screenshot": screenshot,
	})
}

func (c *Client) AssertElementPresent(selector string, screenshot bool) (map[string]interface{}, error) {
	return c.handleAssertion("assert_element_present", map[string]interface{}{
		"selector": selector, "screenshot": screenshot,
	})
}

func (c *Client) AssertElementAbsent(selector string, screenshot bool) (map[string]interface{}, error) {
	return c.handleAssertion("assert_element_absent", map[string]interface{}{
		"selector": selector, "screenshot": screenshot,
	})
}

func (c *Client) AssertElementNotVisible(selector string, screenshot bool) (map[string]interface{}, error) {
	return c.handleAssertion("assert_element_not_visible", map[string]interface{}{
		"selector": selector, "screenshot": screenshot,
	})
}

func (c *Client) AssertTextNotVisible(text, selector string, screenshot bool) (map[string]interface{}, error) {
	if selector == "" {
		selector = "html"
	}
	return c.handleAssertion("assert_text_not_visible", map[string]interface{}{
		"text": text, "selector": selector, "screenshot": screenshot,
	})
}

func (c *Client) AssertTitle(title string, screenshot bool) (map[string]interface{}, error) {
	return c.handleAssertion("assert_title", map[string]interface{}{
		"title": title, "screenshot": screenshot,
	})
}

func (c *Client) AssertURL(urlSubstring string, screenshot bool) (map[string]interface{}, error) {
	return c.handleAssertion("assert_url", map[string]interface{}{
		"url": urlSubstring, "screenshot": screenshot,
	})
}

func (c *Client) AssertAttribute(selector, attribute, value string, screenshot bool) (map[string]interface{}, error) {
	return c.handleAssertion("assert_attribute", map[string]interface{}{
		"selector": selector, "attribute": attribute, "value": value, "screenshot": screenshot,
	})
}

// --- Helpers ---

// saveBase64Result is a private helper to decode and save files returned by the browser
func (c *Client) saveBase64Result(res map[string]interface{}, keyName, outputPath string) (map[string]interface{}, error) {
	if status, ok := res["status"].(string); ok && status == "ok" {
		if val, ok := res[keyName]; ok {
			if b64, ok := val.(string); ok {
				absPath, err := SaveBase64File(b64, outputPath)
				if err != nil {
					return map[string]interface{}{"status": "error", "error": err.Error()}, nil
				}
				return map[string]interface{}{"status": "ok", "path": absPath}, nil
			}
		}
	}
	return res, nil
}
