package format

import (
	"strings"
	"testing"
	"time"

	"api/internal/model"
)

// ─── sanitizeOutput ─────────────────────────────────────────────────────────

func TestSanitizeOutput_PlainText(t *testing.T) {
	input := "Hello, World!"
	got := sanitizeOutput(input)
	if got != input {
		t.Errorf("expected %q unchanged, got %q", input, got)
	}
}

func TestSanitizeOutput_AllowedWhitespace(t *testing.T) {
	input := "line1\nline2\r\n\ttabbed"
	got := sanitizeOutput(input)
	if got != input {
		t.Errorf("newline/tab/CR should pass through unchanged, got %q", got)
	}
}

func TestSanitizeOutput_ANSIEscape(t *testing.T) {
	// ESC character (0x1b) should be replaced with \x1b
	input := "\x1b[31mred text\x1b[0m"
	got := sanitizeOutput(input)
	if strings.Contains(got, "\x1b") {
		t.Errorf("ESC character should be replaced, got %q", got)
	}
	if !strings.Contains(got, `\x1b`) {
		t.Errorf("expected literal \\x1b in output, got %q", got)
	}
}

func TestSanitizeOutput_ControlChars(t *testing.T) {
	// Control characters below 0x20 (except \n, \r, \t) should be escaped
	for _, r := range []rune{0x01, 0x07, 0x08, 0x0c, 0x0e, 0x1a} {
		input := string(r)
		got := sanitizeOutput(input)
		if strings.ContainsRune(got, r) {
			t.Errorf("control char 0x%02x should be escaped, got %q", r, got)
		}
		if !strings.Contains(got, `\x`) {
			t.Errorf("expected \\x escape for 0x%02x, got %q", r, got)
		}
	}
}

func TestSanitizeOutput_DELCharacter(t *testing.T) {
	input := "before\x7fafter"
	got := sanitizeOutput(input)
	if strings.Contains(got, "\x7f") {
		t.Errorf("DEL character should be replaced, got %q", got)
	}
	if !strings.Contains(got, `\x7f`) {
		t.Errorf("expected literal \\x7f in output, got %q", got)
	}
}

func TestSanitizeOutput_Unicode(t *testing.T) {
	input := "Hello, 世界! 🌍"
	got := sanitizeOutput(input)
	if got != input {
		t.Errorf("unicode text should pass through unchanged, got %q", got)
	}
}

func TestSanitizeOutput_Empty(t *testing.T) {
	got := sanitizeOutput("")
	if got != "" {
		t.Errorf("empty string should return empty, got %q", got)
	}
}

func TestSanitizeOutput_MixedContent(t *testing.T) {
	// Mix of safe and unsafe characters
	input := "normal\x1b[0mtext\x07bell"
	got := sanitizeOutput(input)
	if strings.ContainsAny(got, "\x1b\x07") {
		t.Errorf("unsafe chars should be escaped in mixed content, got %q", got)
	}
	if !strings.Contains(got, "normal") || !strings.Contains(got, "text") || !strings.Contains(got, "bell") {
		t.Errorf("safe text should remain in output, got %q", got)
	}
}

// ─── prettyJSON ──────────────────────────────────────────────────────────────

func TestPrettyJSON_ValidObject(t *testing.T) {
	input := `{"key":"value","num":42}`
	got := prettyJSON(input)
	if !strings.Contains(got, "\n") {
		t.Errorf("expected formatted JSON with newlines, got %q", got)
	}
	if !strings.Contains(got, `"key"`) || !strings.Contains(got, `"value"`) {
		t.Errorf("expected JSON content preserved, got %q", got)
	}
}

func TestPrettyJSON_ValidArray(t *testing.T) {
	input := `[1,2,3]`
	got := prettyJSON(input)
	if !strings.Contains(got, "\n") {
		t.Errorf("expected formatted JSON array with newlines, got %q", got)
	}
}

func TestPrettyJSON_AlreadyFormatted(t *testing.T) {
	input := "{\n  \"key\": \"value\"\n}"
	got := prettyJSON(input)
	if !strings.Contains(got, `"key"`) {
		t.Errorf("expected JSON content preserved, got %q", got)
	}
}

func TestPrettyJSON_InvalidJSON(t *testing.T) {
	input := "not json at all"
	got := prettyJSON(input)
	if got != input {
		t.Errorf("invalid JSON should return input unchanged, got %q (want %q)", got, input)
	}
}

func TestPrettyJSON_Empty(t *testing.T) {
	got := prettyJSON("")
	// Empty string is not valid JSON, should return as-is
	if got != "" {
		t.Errorf("empty input should return empty, got %q", got)
	}
}

func TestPrettyJSON_PlainText(t *testing.T) {
	input := "plain text response"
	got := prettyJSON(input)
	if got != input {
		t.Errorf("plain text should be returned as-is, got %q", got)
	}
}

func TestPrettyJSON_HTMLResponse(t *testing.T) {
	input := "<html><body>Not JSON</body></html>"
	got := prettyJSON(input)
	if got != input {
		t.Errorf("HTML should be returned as-is, got %q", got)
	}
}

// ─── getStatusColor ──────────────────────────────────────────────────────────

func TestGetStatusColor_2xx(t *testing.T) {
	for _, code := range []int{200, 201, 204, 299} {
		c := getStatusColor(code)
		if c == nil {
			t.Errorf("expected non-nil color for status %d", code)
		}
		// 2xx should use successColor
		if c != successColor {
			t.Errorf("expected successColor for status %d", code)
		}
	}
}

func TestGetStatusColor_3xx(t *testing.T) {
	for _, code := range []int{301, 302, 304, 307} {
		c := getStatusColor(code)
		if c != redirectColor {
			t.Errorf("expected redirectColor for status %d, got different color", code)
		}
	}
}

func TestGetStatusColor_4xx(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404, 422, 429} {
		c := getStatusColor(code)
		if c != clientErrColor {
			t.Errorf("expected clientErrColor for status %d", code)
		}
	}
}

func TestGetStatusColor_5xx(t *testing.T) {
	for _, code := range []int{500, 502, 503, 504} {
		c := getStatusColor(code)
		if c != serverErrColor {
			t.Errorf("expected serverErrColor for status %d", code)
		}
	}
}

func TestGetStatusColor_100(t *testing.T) {
	// 1xx falls through to the default (serverErrColor)
	c := getStatusColor(100)
	if c != serverErrColor {
		t.Errorf("expected serverErrColor for status 100 (default case)")
	}
}

// ─── PrintHistoryList smoke tests ────────────────────────────────────────────

func TestPrintHistoryList_Empty(t *testing.T) {
	// Should not panic on empty slice
	PrintHistoryList([]model.Request{}, 10)
}

func TestPrintHistoryList_WithItems(t *testing.T) {
	reqs := []model.Request{
		{
			ID:        "abc12345",
			Timestamp: time.Now(),
			Method:    "GET",
			URL:       "https://api.example.com/users",
			Response:  &model.Response{StatusCode: 200, Status: "200 OK", DurationMs: 50},
		},
		{
			ID:        "def67890",
			Timestamp: time.Now(),
			Method:    "POST",
			URL:       "https://api.example.com/users",
			Response:  &model.Response{StatusCode: 201, Status: "201 Created", DurationMs: 80},
		},
	}
	// Should not panic
	PrintHistoryList(reqs, 10)
}

func TestPrintHistoryList_LimitApplied(t *testing.T) {
	reqs := make([]model.Request, 20)
	for i := range reqs {
		reqs[i] = model.Request{
			ID:        "id",
			Timestamp: time.Now(),
			Method:    "GET",
			URL:       "https://example.com",
		}
	}
	// Should not panic with limit less than total
	PrintHistoryList(reqs, 5)
}

func TestPrintHistoryList_NoLimit(t *testing.T) {
	reqs := []model.Request{
		{
			ID:        "abc12345",
			Timestamp: time.Now(),
			Method:    "DELETE",
			URL:       "https://example.com/resource/1",
		},
	}
	// limit=0 means no limit
	PrintHistoryList(reqs, 0)
}

func TestPrintHistoryList_LongURL(t *testing.T) {
	longURL := "https://api.example.com/" + strings.Repeat("very-long-path-segment/", 5)
	reqs := []model.Request{
		{
			ID:        "abc12345",
			Timestamp: time.Now(),
			Method:    "GET",
			URL:       longURL,
		},
	}
	// Should not panic with long URL (truncation logic)
	PrintHistoryList(reqs, 10)
}

// ─── PrintCollectionList smoke tests ─────────────────────────────────────────

func TestPrintCollectionList_Empty(t *testing.T) {
	cols := &model.Collections{Collections: map[string]model.Collection{}}
	PrintCollectionList(cols)
}

func TestPrintCollectionList_WithCollections(t *testing.T) {
	cols := &model.Collections{
		Collections: map[string]model.Collection{
			"my-api": {
				Name: "my-api",
				Requests: []model.SavedRequest{
					{Name: "Get Users", Method: "GET", URL: "https://api.example.com/users"},
				},
			},
			"other": {
				Name:     "other",
				Requests: []model.SavedRequest{},
			},
		},
	}
	PrintCollectionList(cols)
}

// ─── PrintCollectionRequests smoke tests ─────────────────────────────────────

func TestPrintCollectionRequests_Empty(t *testing.T) {
	col := &model.Collection{Name: "empty-col", Requests: []model.SavedRequest{}}
	PrintCollectionRequests(col)
}

func TestPrintCollectionRequests_WithRequests(t *testing.T) {
	col := &model.Collection{
		Name: "test-col",
		Requests: []model.SavedRequest{
			{Name: "Get Users", Method: "GET", URL: "https://api.example.com/users"},
			{Name: "", Method: "POST", URL: "https://api.example.com/users"},
		},
	}
	PrintCollectionRequests(col)
}

// ─── PrintAliasList smoke tests ───────────────────────────────────────────────

func TestPrintAliasList_Empty(t *testing.T) {
	aliases := &model.Aliases{Aliases: map[string]string{}}
	PrintAliasList(aliases)
}

func TestPrintAliasList_WithAliases(t *testing.T) {
	aliases := &model.Aliases{
		Aliases: map[string]string{
			"myapi":    "https://api.example.com",
			"starwars": "https://swapi.tech/api",
		},
	}
	PrintAliasList(aliases)
}

// ─── PrintRequest / PrintRequestDetail smoke tests ───────────────────────────

func TestPrintRequest_NoResponse(t *testing.T) {
	req := &model.Request{
		ID:        "abc12345",
		Timestamp: time.Now(),
		Method:    "GET",
		URL:       "https://example.com",
	}
	PrintRequest(req)
}

func TestPrintRequest_WithResponse(t *testing.T) {
	req := &model.Request{
		ID:        "abc12345",
		Timestamp: time.Now(),
		Method:    "POST",
		URL:       "https://example.com/users",
		Response: &model.Response{
			StatusCode: 201,
			Status:     "201 Created",
		},
	}
	PrintRequest(req)
}

func TestPrintRequestDetail_Full(t *testing.T) {
	req := &model.Request{
		ID:        "abc12345",
		Timestamp: time.Now(),
		Method:    "POST",
		URL:       "https://api.example.com/users",
		Headers:   map[string]string{"Content-Type": "application/json"},
		Body:      `{"name":"Alice"}`,
		Response: &model.Response{
			StatusCode: 201,
			Status:     "201 Created",
			Headers:    map[string]string{"Location": "/users/1"},
			Body:       `{"id":1,"name":"Alice"}`,
			DurationMs: 120,
		},
	}
	PrintRequestDetail(req)
}

func TestPrintRequestDetail_NoBody(t *testing.T) {
	req := &model.Request{
		ID:        "abc12345",
		Timestamp: time.Now(),
		Method:    "GET",
		URL:       "https://api.example.com/users",
	}
	PrintRequestDetail(req)
}

// ─── PrintResponse smoke tests ────────────────────────────────────────────────

func TestPrintResponse_WithHeaders(t *testing.T) {
	resp := &model.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Headers:    map[string]string{"Content-Type": "application/json", "X-Request-Id": "abc"},
		Body:       `{"result":"ok"}`,
		DurationMs: 42,
	}
	PrintResponse(resp, true)
}

func TestPrintResponse_WithoutHeaders(t *testing.T) {
	resp := &model.Response{
		StatusCode: 404,
		Status:     "404 Not Found",
		Headers:    map[string]string{},
		Body:       `{"error":"not found"}`,
		DurationMs: 15,
	}
	PrintResponse(resp, false)
}

func TestPrintResponse_EmptyBody(t *testing.T) {
	resp := &model.Response{
		StatusCode: 204,
		Status:     "204 No Content",
		Headers:    map[string]string{},
		Body:       "",
		DurationMs: 10,
	}
	PrintResponse(resp, false)
}

// ─── PrintSuccess / PrintError smoke tests ───────────────────────────────────

func TestPrintSuccess(t *testing.T) {
	PrintSuccess("operation completed")
}

func TestPrintError(t *testing.T) {
	PrintError("something went wrong")
}

func TestPrintAlias(t *testing.T) {
	PrintAlias("myapi", "https://api.example.com")
}
