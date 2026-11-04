package cassette

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestOpenAPIGeneratesUsableObservedDocument(t *testing.T) {
	header := Header{Format: Format, Target: "https://api.example.com"}
	entries := []Entry{
		{
			ID:       1,
			Request:  Request{Method: "GET", URI: "/v1/users/42?verbose=true"},
			Response: Response{Status: 200, Headers: http.Header{"Content-Type": {"application/json; charset=utf-8"}}, Body: `{"id":42,"name":"Ada","active":true}`, BodyEncoding: "utf8"},
		},
		{
			ID:       2,
			Request:  Request{Method: "GET", URI: "/v1/users/43"},
			Response: Response{Status: 200, Headers: http.Header{"Content-Type": {"application/json"}}, Body: `{"id":43,"name":"Grace","active":false}`, BodyEncoding: "utf8"},
		},
		{
			ID:       3,
			Request:  Request{Method: "POST", URI: "/v1/users", Headers: http.Header{"Content-Type": {"application/json"}}, Body: `{"name":"Ada","password":"[REDACTED]"}`, BodyEncoding: "utf8"},
			Response: Response{Status: 201, Headers: http.Header{"Content-Type": {"application/json"}}, Body: `{"id":44,"name":"Ada"}`, BodyEncoding: "utf8"},
		},
	}
	document := OpenAPI(header, entries, "Example API", "1.2.3")
	if document["openapi"] != "3.1.0" {
		t.Fatalf("unexpected OpenAPI version %v", document["openapi"])
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	paths := decoded["paths"].(map[string]any)
	userPath, ok := paths["/v1/users/{id}"].(map[string]any)
	if !ok {
		t.Fatalf("numeric ID was not generalized: %v", paths)
	}
	get := userPath["get"].(map[string]any)
	parameters := get["parameters"].([]any)
	var pathRequired, verboseFound bool
	for _, raw := range parameters {
		parameter := raw.(map[string]any)
		if parameter["in"] == "path" && parameter["name"] == "id" {
			pathRequired = parameter["required"] == true
		}
		if parameter["name"] == "verbose" {
			verboseFound = true
			if parameter["required"] != false {
				t.Fatal("query present in only one observation was marked required")
			}
		}
	}
	if !pathRequired || !verboseFound {
		t.Fatalf("parameters missing: %v", parameters)
	}
	response200 := get["responses"].(map[string]any)["200"].(map[string]any)
	content := response200["content"].(map[string]any)["application/json"].(map[string]any)
	schema := content["schema"].(map[string]any)
	if schema["type"] != "object" || content["example"] == nil {
		t.Fatalf("response schema/example missing: %v", content)
	}
	collection := paths["/v1/users"].(map[string]any)
	post := collection["post"].(map[string]any)
	if post["requestBody"] == nil || post["responses"].(map[string]any)["201"] == nil {
		t.Fatalf("POST body or 201 response missing: %v", post)
	}
	if post["operationId"] != "post_v1_users" {
		t.Fatalf("unexpected operationId %v", post["operationId"])
	}
	if get["x-equal-observed-interactions"].([]any)[0] != float64(1) {
		t.Fatal("interaction provenance missing")
	}
}

func TestOpenAPIBinaryAndUUID(t *testing.T) {
	entries := []Entry{{
		ID:       1,
		Request:  Request{Method: "GET", URI: "/assets/123e4567-e89b-12d3-a456-426614174000"},
		Response: Response{Status: 200, Headers: http.Header{"Content-Type": {"application/octet-stream"}}, Body: "AAEC", BodyEncoding: "base64"},
	}}
	document := OpenAPI(Header{Target: "http://localhost"}, entries, "", "")
	paths := document["paths"].(map[string]any)
	if paths["/assets/{id}"] == nil {
		t.Fatalf("UUID path parameter not inferred: %v", paths)
	}
	encoded, _ := json.Marshal(document)
	var decoded map[string]any
	_ = json.Unmarshal(encoded, &decoded)
	get := decoded["paths"].(map[string]any)["/assets/{id}"].(map[string]any)["get"].(map[string]any)
	response := get["responses"].(map[string]any)["200"].(map[string]any)
	content := response["content"].(map[string]any)["application/octet-stream"].(map[string]any)
	if content["schema"].(map[string]any)["contentEncoding"] != "base64" {
		t.Fatalf("binary encoding missing: %v", content)
	}
}

func TestOpenAPIPreservesAlternativeResponses(t *testing.T) {
	entries := []Entry{
		{ID: 1, Request: Request{Method: "GET", URI: "/job"}, Response: Response{Status: 200, Body: `{"state":"queued"}`, BodyEncoding: "utf8"}},
		{ID: 2, Request: Request{Method: "GET", URI: "/job"}, Response: Response{Status: 200, Body: `{"state":"done"}`, BodyEncoding: "utf8"}},
	}
	document := OpenAPI(Header{}, entries, "", "")
	get := document["paths"].(map[string]any)["/job"].(map[string]any)["get"].(map[string]any)
	response := get["responses"].(map[string]any)["200"].(map[string]any)
	if response["content"] == nil || response["x-equal-observed-alternatives"] == nil {
		t.Fatalf("alternative response was discarded: %v", response)
	}
}
