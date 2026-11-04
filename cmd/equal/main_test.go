package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d: %s", code, stderr.String())
	}
	if stdout.String() != "EQUAL 0.2.0\n" {
		t.Fatalf("version output %q", stdout.String())
	}
}

func TestUsageAndUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), nil, &stdout, &stderr); code != 2 {
		t.Fatalf("empty invocation exit code %d", code)
	}
	if code := run(context.Background(), []string{"unknown"}, &stdout, &stderr); code != 2 {
		t.Fatalf("unknown command exit code %d", code)
	}
}

func TestRequiredFlags(t *testing.T) {
	for _, command := range []string{"record", "replay", "verify", "inspect", "export"} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), []string{command}, &stdout, &stderr); code != 1 {
			t.Fatalf("%s exit code %d, stderr %s", command, code, stderr.String())
		}
	}
}

func TestParseHeaders(t *testing.T) {
	headers, err := parseHeaders([]string{"Authorization: Bearer test", "X-Empty:"})
	if err != nil {
		t.Fatal(err)
	}
	if headers.Get("Authorization") != "Bearer test" {
		t.Fatal("header value changed")
	}
	if _, ok := headers["X-Empty"]; !ok {
		t.Fatal("empty header was dropped")
	}
	if _, err := parseHeaders([]string{"broken"}); err == nil {
		t.Fatal("invalid header was accepted")
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"help"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "record") {
		t.Fatalf("help failed: %d %s", code, stdout.String())
	}
}
