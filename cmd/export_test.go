package cmd

import (
	"strings"
	"testing"

	"api/internal/model"
	"api/internal/storage"
)

// ─── isJSON ───────────────────────────────────────────────────────────────────

func TestIsJSON_Object(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{`{"key":"value"}`, true},
		{`{"a":1,"b":2}`, true},
		{`{}`, true},
		{`[]`, true},
		{`[1,2,3]`, true},
		{`[{"id":1}]`, true},
		// With whitespace
		{"  { \"key\": \"value\" }  ", true},
		{"  [ 1, 2 ]  ", true},
		// Non-JSON
		{"plain text", false},
		{"", false},
		{"<html></html>", false},
		{"{incomplete", false},
		{"incomplete}", false},
		{"[incomplete", false},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := isJSON(tc.input)
			if got != tc.want {
				t.Errorf("isJSON(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ─── parsePostmanURL ──────────────────────────────────────────────────────────

func TestParsePostmanURL_SimpleHTTPS(t *testing.T) {
	pu := parsePostmanURL("https://api.example.com/users")
	if pu.Raw != "https://api.example.com/users" {
		t.Errorf("Raw mismatch: %q", pu.Raw)
	}
	if pu.Protocol != "https" {
		t.Errorf("Protocol mismatch: %q", pu.Protocol)
	}
	if len(pu.Host) == 0 {
		t.Error("expected non-empty host parts")
	}
	if strings.Join(pu.Host, ".") != "api.example.com" {
		t.Errorf("Host parts mismatch: %v", pu.Host)
	}
}

func TestParsePostmanURL_WithPath(t *testing.T) {
	pu := parsePostmanURL("https://api.example.com/v1/users/123")
	if len(pu.Path) != 3 {
		t.Errorf("expected 3 path segments, got %d: %v", len(pu.Path), pu.Path)
	}
	if pu.Path[0] != "v1" || pu.Path[1] != "users" || pu.Path[2] != "123" {
		t.Errorf("unexpected path segments: %v", pu.Path)
	}
}

func TestParsePostmanURL_WithQueryParams(t *testing.T) {
	pu := parsePostmanURL("https://api.example.com/search?q=hello&page=1")
	if len(pu.Query) == 0 {
		t.Error("expected query params, got none")
	}
	found := map[string]string{}
	for _, qp := range pu.Query {
		found[qp.Key] = qp.Value
	}
	if found["q"] != "hello" {
		t.Errorf("expected q=hello, got %q", found["q"])
	}
	if found["page"] != "1" {
		t.Errorf("expected page=1, got %q", found["page"])
	}
}

func TestParsePostmanURL_NoPath(t *testing.T) {
	pu := parsePostmanURL("https://example.com")
	if pu.Raw != "https://example.com" {
		t.Errorf("Raw mismatch: %q", pu.Raw)
	}
	if len(pu.Path) != 0 {
		t.Errorf("expected no path segments, got %v", pu.Path)
	}
}

func TestParsePostmanURL_HTTP(t *testing.T) {
	pu := parsePostmanURL("http://localhost:8080/api")
	if pu.Protocol != "http" {
		t.Errorf("Protocol mismatch: %q", pu.Protocol)
	}
}

func TestParsePostmanURL_TrailingSlash(t *testing.T) {
	pu := parsePostmanURL("https://api.example.com/users/")
	// Trailing slash produces an empty segment which should be filtered
	for _, seg := range pu.Path {
		if seg == "" {
			t.Error("empty path segment should be filtered out")
		}
	}
}

// ─── buildPostmanRequest ──────────────────────────────────────────────────────

func TestBuildPostmanRequest_GET_NoBody(t *testing.T) {
	pr := buildPostmanRequest("GET", "https://api.example.com/users", nil, "")
	if pr.Method != "GET" {
		t.Errorf("Method mismatch: %q", pr.Method)
	}
	if pr.Body != nil {
		t.Errorf("expected no body for GET, got %+v", pr.Body)
	}
	if pr.URL.Raw != "https://api.example.com/users" {
		t.Errorf("URL.Raw mismatch: %q", pr.URL.Raw)
	}
}

func TestBuildPostmanRequest_POST_JSONBody(t *testing.T) {
	pr := buildPostmanRequest("POST", "https://api.example.com/users",
		map[string]string{"Content-Type": "application/json"},
		`{"name":"Alice"}`)
	if pr.Method != "POST" {
		t.Errorf("Method mismatch: %q", pr.Method)
	}
	if pr.Body == nil {
		t.Fatal("expected body, got nil")
	}
	if pr.Body.Mode != "raw" {
		t.Errorf("expected mode 'raw', got %q", pr.Body.Mode)
	}
	if pr.Body.Raw != `{"name":"Alice"}` {
		t.Errorf("body Raw mismatch: %q", pr.Body.Raw)
	}
	if pr.Body.Options == nil || pr.Body.Options.Raw.Language != "json" {
		t.Error("expected JSON language in body options")
	}
}

func TestBuildPostmanRequest_POST_TextBody(t *testing.T) {
	pr := buildPostmanRequest("POST", "https://api.example.com/data", nil, "plain text body")
	if pr.Body == nil {
		t.Fatal("expected body, got nil")
	}
	if pr.Body.Options == nil || pr.Body.Options.Raw.Language != "text" {
		t.Error("expected 'text' language for non-JSON body")
	}
}

func TestBuildPostmanRequest_UppercaseMethod(t *testing.T) {
	// Method should be uppercased
	pr := buildPostmanRequest("delete", "https://api.example.com/resource/1", nil, "")
	if pr.Method != "DELETE" {
		t.Errorf("expected uppercase method DELETE, got %q", pr.Method)
	}
}

func TestBuildPostmanRequest_Headers(t *testing.T) {
	headers := map[string]string{
		"Accept":       "application/json",
		"X-Request-ID": "req-123",
	}
	pr := buildPostmanRequest("GET", "https://api.example.com", headers, "")
	if len(pr.Header) != 2 {
		t.Errorf("expected 2 headers, got %d", len(pr.Header))
	}
	found := map[string]string{}
	for _, h := range pr.Header {
		found[h.Key] = h.Value
	}
	if found["Accept"] != "application/json" {
		t.Errorf("unexpected Accept header: %q", found["Accept"])
	}
}

// ─── savedRequestToPostmanItem ────────────────────────────────────────────────

func TestSavedRequestToPostmanItem_WithName(t *testing.T) {
	req := model.SavedRequest{
		Name:    "Get All Users",
		Method:  "GET",
		URL:     "https://api.example.com/users",
		Headers: map[string]string{},
		Body:    "",
	}
	item := savedRequestToPostmanItem(req)
	if item.Name != "Get All Users" {
		t.Errorf("Name mismatch: %q", item.Name)
	}
	if item.Request.Method != "GET" {
		t.Errorf("Method mismatch: %q", item.Request.Method)
	}
}

func TestSavedRequestToPostmanItem_WithoutName(t *testing.T) {
	req := model.SavedRequest{
		Name:    "",
		Method:  "POST",
		URL:     "https://api.example.com/users",
		Headers: map[string]string{},
		Body:    "",
	}
	item := savedRequestToPostmanItem(req)
	// When name is empty, it should be generated from method + url
	if item.Name == "" {
		t.Error("expected generated name for unnamed request")
	}
	if !strings.Contains(item.Name, "POST") {
		t.Errorf("expected name to contain method, got %q", item.Name)
	}
}

// ─── historyRequestToPostmanItem ──────────────────────────────────────────────

func TestHistoryRequestToPostmanItem(t *testing.T) {
	req := model.Request{
		ID:     "abc12345",
		Method: "DELETE",
		URL:    "https://api.example.com/users/42",
	}
	item := historyRequestToPostmanItem(req)
	if !strings.Contains(item.Name, "DELETE") {
		t.Errorf("expected name to contain method, got %q", item.Name)
	}
	if !strings.Contains(item.Name, "https://api.example.com/users/42") {
		t.Errorf("expected name to contain URL, got %q", item.Name)
	}
	if item.Request.Method != "DELETE" {
		t.Errorf("Method mismatch: %q", item.Request.Method)
	}
}

// ─── buildPostmanFromCollection / buildPostmanFromHistory (integration) ───────

func TestBuildPostmanFromCollection_Success(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	defer store.Close()

	store.CreateCollection("test-col")
	store.AddToCollection("test-col", model.SavedRequest{
		Name:    "List Users",
		Method:  "GET",
		URL:     "https://api.example.com/users",
		Headers: map[string]string{},
	})
	store.AddToCollection("test-col", model.SavedRequest{
		Name:    "Create User",
		Method:  "POST",
		URL:     "https://api.example.com/users",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    `{"name":"Alice"}`,
	})

	pc, err := buildPostmanFromCollection(store, "test-col")
	if err != nil {
		t.Fatalf("buildPostmanFromCollection: %v", err)
	}
	if pc.Info.Name != "test-col" {
		t.Errorf("collection name mismatch: %q", pc.Info.Name)
	}
	if len(pc.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(pc.Items))
	}
	if !strings.Contains(pc.Info.Schema, "postman") {
		t.Errorf("expected Postman schema URL, got %q", pc.Info.Schema)
	}
}

func TestBuildPostmanFromCollection_NotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	defer store.Close()

	_, err = buildPostmanFromCollection(store, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent collection, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestBuildPostmanFromHistory_Empty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	defer store.Close()

	pc, err := buildPostmanFromHistory(store)
	if err != nil {
		t.Fatalf("buildPostmanFromHistory: %v", err)
	}
	if len(pc.Items) != 0 {
		t.Errorf("expected 0 items for empty history, got %d", len(pc.Items))
	}
	if !strings.Contains(pc.Info.Name, "History Export") {
		t.Errorf("expected history export name, got %q", pc.Info.Name)
	}
}

func TestBuildPostmanFromAllCollections_Empty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	defer store.Close()

	pc, err := buildPostmanFromAllCollections(store)
	if err != nil {
		t.Fatalf("buildPostmanFromAllCollections: %v", err)
	}
	if len(pc.Items) != 0 {
		t.Errorf("expected 0 items for empty collections, got %d", len(pc.Items))
	}
}

func TestBuildPostmanFromAllCollections_Single(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	defer store.Close()

	store.CreateCollection("my-api")
	store.AddToCollection("my-api", model.SavedRequest{
		Name:    "Health Check",
		Method:  "GET",
		URL:     "https://api.example.com/health",
		Headers: map[string]string{},
	})

	pc, err := buildPostmanFromAllCollections(store)
	if err != nil {
		t.Fatalf("buildPostmanFromAllCollections: %v", err)
	}
	// Single collection → use its name
	if pc.Info.Name != "my-api" {
		t.Errorf("expected collection name 'my-api', got %q", pc.Info.Name)
	}
	if len(pc.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(pc.Items))
	}
}

func TestBuildPostmanFromAllCollections_Multiple(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	defer store.Close()

	store.CreateCollection("col-a")
	store.AddToCollection("col-a", model.SavedRequest{Method: "GET", URL: "https://a.com", Headers: map[string]string{}})
	store.AddToCollection("col-a", model.SavedRequest{Method: "POST", URL: "https://a.com", Headers: map[string]string{}})
	store.CreateCollection("col-b")
	store.AddToCollection("col-b", model.SavedRequest{Method: "DELETE", URL: "https://b.com", Headers: map[string]string{}})

	pc, err := buildPostmanFromAllCollections(store)
	if err != nil {
		t.Fatalf("buildPostmanFromAllCollections: %v", err)
	}
	// Multiple collections → generic name
	if pc.Info.Name != "apicli Export" {
		t.Errorf("expected 'apicli Export', got %q", pc.Info.Name)
	}
	if len(pc.Items) != 3 {
		t.Errorf("expected 3 total items, got %d", len(pc.Items))
	}
}

func TestBuildPostmanFromHistory_WithRequests(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	defer store.Close()

	import_time := model.Request{
		ID:      "hist0001",
		Method:  "GET",
		URL:     "https://api.example.com/users",
		Headers: map[string]string{},
	}
	store.AddToHistory(import_time)

	pc, err := buildPostmanFromHistory(store)
	if err != nil {
		t.Fatalf("buildPostmanFromHistory: %v", err)
	}
	if len(pc.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(pc.Items))
	}
	if pc.Items[0].Request.Method != "GET" {
		t.Errorf("method mismatch: %q", pc.Items[0].Request.Method)
	}
}
