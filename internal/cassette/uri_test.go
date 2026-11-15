package cassette

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRequestURI(t *testing.T) {
	for _, tc := range []struct {
		uri   string
		valid bool
	}{
		{"/v1/a%zz", false}, {"/v1/a b", false},
		{"/v1/users/42?include=profile", true},
		{"https://example.com/v1/users?include=profile", true},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cassette.jsonl")
			recorder, err := NewRecorder(path, Header{Format: Format}, false, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := recorder.Write(Entry{ID: 7, Request: Request{Method: "GET", URI: tc.uri}, Response: Response{Status: 200}}); err != nil {
				t.Fatal(err)
			}
			if err := recorder.Close(); err != nil {
				t.Fatal(err)
			}
			_, entries, err := Load(path)
			if tc.valid {
				if err != nil || len(entries) != 1 {
					t.Fatalf("valid URI: entries=%v err=%v", entries, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "interaction 7") || !strings.Contains(err.Error(), tc.uri) {
				t.Fatalf("expected interaction and URI in error, got %v", err)
			}
		})
	}
}
