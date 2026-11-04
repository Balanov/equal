package cassette

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBodyRoundTrip(t *testing.T) {
	cases := [][]byte{nil, []byte("plain"), {0, 0xff, 1, 2}, []byte("{\"json\":true}")}
	for _, input := range cases {
		body, encoding := SetBody(input)
		output, err := BodyBytes(body, encoding)
		if err != nil {
			t.Fatal(err)
		}
		if string(output) != string(input) {
			t.Fatalf("round trip changed %v into %v", input, output)
		}
	}
	body, encoding := SetBody([]byte{0xff})
	if encoding != "base64" || body != base64.StdEncoding.EncodeToString([]byte{0xff}) {
		t.Fatal("binary body was not preserved as base64")
	}
}

func TestRedactRequestAndResponse(t *testing.T) {
	request := Request{
		Method: "POST", URI: "/login?access_token=query-secret&ok=1",
		Headers: http.Header{"Authorization": {"Bearer header-secret"}, "Content-Type": {"application/json"}},
		Body:    `{"password":"body-secret","nested":{"refresh_token":"nested-secret"},"ok":1}`, BodyEncoding: "utf8",
	}
	RedactRequest(&request, DefaultRedactions)
	if got := request.Headers.Get("Authorization"); got != Redacted {
		t.Fatalf("authorization = %q", got)
	}
	if strings.Contains(request.URI, "query-secret") || strings.Contains(request.Body, "body-secret") || strings.Contains(request.Body, "nested-secret") {
		t.Fatalf("request contains a secret: %+v", request)
	}
	if !strings.Contains(request.URI, "ok=1") || !strings.Contains(request.Body, `"ok":1`) {
		t.Fatalf("non-secret data changed: %+v", request)
	}
	response := Response{Status: 200, Headers: http.Header{"Set-Cookie": {"secret"}}, Body: `{"access_token":"secret","ok":true}`, BodyEncoding: "utf8"}
	RedactResponse(&response, DefaultRedactions)
	if strings.Contains(response.Body, "secret") || response.Headers.Get("Set-Cookie") != Redacted {
		t.Fatalf("response was not redacted: %+v", response)
	}
}

func TestRedactionDoesNotChangeBinary(t *testing.T) {
	request := Request{Body: "/wA=", BodyEncoding: "base64"}
	RedactRequest(&request, DefaultRedactions)
	if request.Body != "/wA=" || request.BodyEncoding != "base64" {
		t.Fatal("binary payload changed")
	}
}

func TestKeyCanonicalizesJSONAndQuery(t *testing.T) {
	a := Request{Method: "POST", URI: "/v1/items?b=2&a=1", Body: `{"x":1,"y":[2,3]}`, BodyEncoding: "utf8"}
	b := Request{Method: "POST", URI: "/v1/items?a=1&b=2", Body: "{\n  \"y\": [2, 3], \"x\": 1\n}", BodyEncoding: "utf8"}
	if Key(a, nil) != Key(b, nil) {
		t.Fatal("semantically identical requests have different keys")
	}
	b.Body = `{"x":1,"y":[3,2]}`
	if Key(a, nil) == Key(b, nil) {
		t.Fatal("array order was incorrectly ignored")
	}
}

func TestKeyUsesOnlySelectedHeaders(t *testing.T) {
	a := Request{Method: "GET", URI: "/", Headers: http.Header{"Accept": {"application/json"}, "X-Request-Id": {"a"}}}
	b := Request{Method: "GET", URI: "/", Headers: http.Header{"Accept": {"application/json"}, "X-Request-Id": {"b"}}}
	if Key(a, nil) != Key(b, nil) {
		t.Fatal("unselected headers affected matching")
	}
	b.Headers.Set("Accept", "text/plain")
	if Key(a, []string{"accept"}) == Key(b, []string{"accept"}) {
		t.Fatal("selected header did not affect matching")
	}
}

func TestCassetteLoadSortsAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.jsonl")
	recorder, err := NewRecorder(path, Header{Format: Format, CreatedAt: time.Now().UTC().Format(time.RFC3339), Target: "https://example.com"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint64{2, 1} {
		if err := recorder.Write(Entry{ID: id, Request: Request{Method: "GET", URI: "/"}, Response: Response{Status: 200}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}
	_, entries, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].ID != 1 || entries[1].ID != 2 {
		t.Fatal("entries were not restored in request order")
	}
	if _, err := NewRecorder(path, Header{Format: Format}, false, false); err == nil {
		t.Fatal("existing cassette was overwritten without permission")
	}
}

func TestLoadRejectsBrokenCassette(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.jsonl")
	if err := os.WriteFile(path, []byte("{\"format\":\"other/v1\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path); err == nil {
		t.Fatal("unsupported format was accepted")
	}
}

func TestCompareSemanticJSONAndIgnorePointers(t *testing.T) {
	expected := Response{Status: 200, Headers: http.Header{"Content-Type": {"application/json; charset=UTF-8"}}, Body: `{"id":"a","value":1,"items":[{"request":"a","ok":true}]}`, BodyEncoding: "utf8"}
	actual := Response{Status: 200, Headers: http.Header{"Content-Type": {"application/json; charset=utf-8"}}, Body: `{"items":[{"ok":true,"request":"b"}],"value":1.0,"id":"b"}`, BodyEncoding: "utf8"}
	differences := Compare(expected, actual, []string{"/id", "/items/*/request"}, []string{"Content-Type", "Location"})
	if len(differences) != 0 {
		t.Fatalf("unexpected differences: %+v", differences)
	}
	actual.Body = `{"id":"b","value":2,"items":[]}`
	differences = Compare(expected, actual, []string{"/id"}, []string{"Content-Type"})
	if len(differences) < 2 {
		t.Fatalf("drift was not explained: %+v", differences)
	}
}

func TestCompareBinaryAndStatus(t *testing.T) {
	expected := Response{Status: 200, Body: "AAE=", BodyEncoding: "base64"}
	actual := Response{Status: 201, Body: "AAI=", BodyEncoding: "base64"}
	differences := Compare(expected, actual, nil, nil)
	if len(differences) != 2 {
		t.Fatalf("expected status and body differences, got %+v", differences)
	}
}

func TestManyCanonicalKeys(t *testing.T) {
	for index := 0; index < 500; index++ {
		a := Request{Method: "POST", URI: "/v1?x=1&y=2", Body: `{"n":1,"s":"x"}`, BodyEncoding: "utf8"}
		b := Request{Method: "POST", URI: "/v1?y=2&x=1", Body: `{"s":"x","n":1}`, BodyEncoding: "utf8"}
		if Key(a, nil) != Key(b, nil) {
			t.Fatalf("iteration %d failed", index)
		}
	}
}
