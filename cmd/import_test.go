package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

// ─── resolvePostmanURL ────────────────────────────────────────────────────────

func TestResolvePostmanURL_PlainString(t *testing.T) {
	raw := json.RawMessage(`"https://api.example.com/users"`)
	got := resolvePostmanURL(raw)
	if got != "https://api.example.com/users" {
		t.Errorf("expected plain string URL, got %q", got)
	}
}

func TestResolvePostmanURL_ObjectWithRaw(t *testing.T) {
	raw := json.RawMessage(`{
		"raw": "https://api.example.com/v1/users",
		"protocol": "https",
		"host": ["api", "example", "com"],
		"path": ["v1", "users"]
	}`)
	got := resolvePostmanURL(raw)
	if got != "https://api.example.com/v1/users" {
		t.Errorf("expected raw URL from object, got %q", got)
	}
}

func TestResolvePostmanURL_ObjectWithoutRaw_Reconstruct(t *testing.T) {
	raw := json.RawMessage(`{
		"protocol": "https",
		"host": ["api", "example", "com"],
		"path": ["users", "123"]
	}`)
	got := resolvePostmanURL(raw)
	if got == "" {
		t.Error("expected reconstructed URL, got empty")
	}
	if !strings.Contains(got, "api.example.com") {
		t.Errorf("expected host in URL, got %q", got)
	}
	if !strings.Contains(got, "users") {
		t.Errorf("expected path in URL, got %q", got)
	}
}

func TestResolvePostmanURL_ObjectHostOnly(t *testing.T) {
	raw := json.RawMessage(`{
		"protocol": "https",
		"host": ["example", "com"]
	}`)
	got := resolvePostmanURL(raw)
	if got == "" {
		t.Error("expected URL with just host, got empty")
	}
	if !strings.Contains(got, "example.com") {
		t.Errorf("expected 'example.com' in URL, got %q", got)
	}
}

func TestResolvePostmanURL_ObjectDefaultProtocol(t *testing.T) {
	// No protocol → default to https
	raw := json.RawMessage(`{
		"host": ["api", "example", "com"]
	}`)
	got := resolvePostmanURL(raw)
	if !strings.HasPrefix(got, "https://") {
		t.Errorf("expected https:// default protocol, got %q", got)
	}
}

func TestResolvePostmanURL_Empty(t *testing.T) {
	got := resolvePostmanURL(json.RawMessage{})
	if got != "" {
		t.Errorf("expected empty string for empty raw message, got %q", got)
	}
}

func TestResolvePostmanURL_InvalidJSON(t *testing.T) {
	// Neither a string nor an object
	raw := json.RawMessage(`12345`)
	// Should not panic; returns empty or reconstructed
	_ = resolvePostmanURL(raw)
}

func TestResolvePostmanURL_NullValue(t *testing.T) {
	raw := json.RawMessage(`null`)
	// null is not a string or an object with host, so reconstruct returns ""
	_ = resolvePostmanURL(raw)
}

// ─── flattenPostmanItems ──────────────────────────────────────────────────────

func makeLeafItem(name, method, url string) postmanItemImport {
	rawURL, _ := json.Marshal(url)
	return postmanItemImport{
		Name: name,
		Request: &postmanRequestImport{
			Method: method,
			Header: []postmanHeader{},
			URL:    json.RawMessage(rawURL),
		},
	}
}

func makeFolderItem(name string, children []postmanItemImport) postmanItemImport {
	return postmanItemImport{
		Name:  name,
		Items: children,
	}
}

func TestFlattenPostmanItems_Empty(t *testing.T) {
	result := flattenPostmanItems([]postmanItemImport{}, "")
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestFlattenPostmanItems_FlatLeaves(t *testing.T) {
	items := []postmanItemImport{
		makeLeafItem("Get Users", "GET", "https://api.example.com/users"),
		makeLeafItem("Create User", "POST", "https://api.example.com/users"),
		makeLeafItem("Delete User", "DELETE", "https://api.example.com/users/1"),
	}
	result := flattenPostmanItems(items, "")
	if len(result) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(result))
	}
	if result[0].Name != "Get Users" {
		t.Errorf("name mismatch: %q", result[0].Name)
	}
	if result[1].Method != "POST" {
		t.Errorf("method mismatch: %q", result[1].Method)
	}
}

func TestFlattenPostmanItems_NestedFolder(t *testing.T) {
	items := []postmanItemImport{
		makeLeafItem("Health Check", "GET", "https://api.example.com/health"),
		makeFolderItem("Users", []postmanItemImport{
			makeLeafItem("List", "GET", "https://api.example.com/users"),
			makeLeafItem("Create", "POST", "https://api.example.com/users"),
		}),
	}
	result := flattenPostmanItems(items, "")
	if len(result) != 3 {
		t.Fatalf("expected 3 requests (1 top-level + 2 from folder), got %d", len(result))
	}
	// Nested items should have folder name prepended
	if result[1].Name != "Users / List" {
		t.Errorf("expected 'Users / List', got %q", result[1].Name)
	}
	if result[2].Name != "Users / Create" {
		t.Errorf("expected 'Users / Create', got %q", result[2].Name)
	}
}

func TestFlattenPostmanItems_DeepNesting(t *testing.T) {
	items := []postmanItemImport{
		makeFolderItem("Auth", []postmanItemImport{
			makeFolderItem("OAuth", []postmanItemImport{
				makeLeafItem("Token", "POST", "https://auth.example.com/token"),
			}),
		}),
	}
	result := flattenPostmanItems(items, "")
	if len(result) != 1 {
		t.Fatalf("expected 1 request, got %d", len(result))
	}
	if result[0].Name != "Auth / OAuth / Token" {
		t.Errorf("expected 'Auth / OAuth / Token', got %q", result[0].Name)
	}
}

func TestFlattenPostmanItems_FolderWithNoRequests(t *testing.T) {
	// Empty folder — no leaf items
	items := []postmanItemImport{
		makeFolderItem("EmptyFolder", []postmanItemImport{}),
	}
	result := flattenPostmanItems(items, "")
	if len(result) != 0 {
		t.Errorf("expected 0 results from empty folder, got %d", len(result))
	}
}

func TestFlattenPostmanItems_WithPrefix(t *testing.T) {
	items := []postmanItemImport{
		makeLeafItem("Get", "GET", "https://example.com"),
	}
	result := flattenPostmanItems(items, "ParentFolder / ")
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Name != "ParentFolder / Get" {
		t.Errorf("expected prefixed name, got %q", result[0].Name)
	}
}

// ─── convertPostmanRequest ────────────────────────────────────────────────────

func TestConvertPostmanRequest_GET(t *testing.T) {
	rawURL, _ := json.Marshal("https://api.example.com/users")
	pr := &postmanRequestImport{
		Method: "get",
		Header: []postmanHeader{},
		URL:    json.RawMessage(rawURL),
	}
	req := convertPostmanRequest("Get Users", pr)

	if req.Name != "Get Users" {
		t.Errorf("Name mismatch: %q", req.Name)
	}
	if req.Method != "GET" {
		t.Errorf("Method should be uppercased: %q", req.Method)
	}
	if req.URL != "https://api.example.com/users" {
		t.Errorf("URL mismatch: %q", req.URL)
	}
	if req.Body != "" {
		t.Errorf("expected no body, got %q", req.Body)
	}
}

func TestConvertPostmanRequest_WithHeaders(t *testing.T) {
	rawURL, _ := json.Marshal("https://api.example.com/users")
	pr := &postmanRequestImport{
		Method: "POST",
		Header: []postmanHeader{
			{Key: "Content-Type", Value: "application/json"},
			{Key: "Accept", Value: "application/json"},
			{Key: "", Value: "should-be-skipped"},
		},
		URL: json.RawMessage(rawURL),
	}
	req := convertPostmanRequest("Create User", pr)

	if req.Headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type header mismatch: %q", req.Headers["Content-Type"])
	}
	if req.Headers["Accept"] != "application/json" {
		t.Errorf("Accept header mismatch: %q", req.Headers["Accept"])
	}
	// Empty key should be skipped
	if _, exists := req.Headers[""]; exists {
		t.Error("empty key header should be skipped")
	}
}

func TestConvertPostmanRequest_WithRawBody(t *testing.T) {
	rawURL, _ := json.Marshal("https://api.example.com/users")
	pr := &postmanRequestImport{
		Method: "POST",
		Header: []postmanHeader{},
		URL:    json.RawMessage(rawURL),
		Body: &postmanBody{
			Mode: "raw",
			Raw:  `{"name":"Alice","email":"alice@example.com"}`,
		},
	}
	req := convertPostmanRequest("Create User", pr)

	if req.Body != `{"name":"Alice","email":"alice@example.com"}` {
		t.Errorf("body mismatch: %q", req.Body)
	}
}

func TestConvertPostmanRequest_NonRawBodySkipped(t *testing.T) {
	rawURL, _ := json.Marshal("https://api.example.com/upload")
	pr := &postmanRequestImport{
		Method: "POST",
		Header: []postmanHeader{},
		URL:    json.RawMessage(rawURL),
		Body: &postmanBody{
			Mode: "formdata",
			Raw:  "should be ignored",
		},
	}
	req := convertPostmanRequest("Upload", pr)
	if req.Body != "" {
		t.Errorf("non-raw body mode should be ignored, got %q", req.Body)
	}
}

func TestConvertPostmanRequest_NilBody(t *testing.T) {
	rawURL, _ := json.Marshal("https://api.example.com/users")
	pr := &postmanRequestImport{
		Method: "GET",
		Header: []postmanHeader{},
		URL:    json.RawMessage(rawURL),
		Body:   nil,
	}
	req := convertPostmanRequest("Get Users", pr)
	if req.Body != "" {
		t.Errorf("nil body should result in empty body, got %q", req.Body)
	}
}

func TestConvertPostmanRequest_BodyModeCaseInsensitive(t *testing.T) {
	rawURL, _ := json.Marshal("https://api.example.com/data")
	pr := &postmanRequestImport{
		Method: "POST",
		Header: []postmanHeader{},
		URL:    json.RawMessage(rawURL),
		Body: &postmanBody{
			Mode: "RAW", // uppercase
			Raw:  `{"key":"value"}`,
		},
	}
	req := convertPostmanRequest("Post Data", pr)
	if req.Body != `{"key":"value"}` {
		t.Errorf("case-insensitive mode match failed, got body %q", req.Body)
	}
}

func TestConvertPostmanRequest_URLAsObject(t *testing.T) {
	urlObj := map[string]interface{}{
		"raw":      "https://api.example.com/users",
		"protocol": "https",
		"host":     []string{"api", "example", "com"},
		"path":     []string{"users"},
	}
	rawURL, _ := json.Marshal(urlObj)
	pr := &postmanRequestImport{
		Method: "GET",
		Header: []postmanHeader{},
		URL:    json.RawMessage(rawURL),
	}
	req := convertPostmanRequest("Get Users", pr)
	if req.URL != "https://api.example.com/users" {
		t.Errorf("URL from object mismatch: %q", req.URL)
	}
}
