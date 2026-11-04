package lab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Balanov/equal/internal/cassette"
)

func testHeader(target string) cassette.Header {
	return cassette.Header{Format: cassette.Format, CreatedAt: time.Now().UTC().Format(time.RFC3339), Target: target, Redactions: cassette.NormalizedNames(cassette.DefaultRedactions)}
}

func TestRecordRedactsCassetteNotLiveResponseAndReplayIsOffline(t *testing.T) {
	var upstreamAuth, upstreamPassword string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		var input map[string]any
		_ = json.Unmarshal(body, &input)
		upstreamPassword, _ = input["password"].(string)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"access_token":"live-server-token","ok":true}`))
	}))
	target, _ := url.Parse(upstream.URL)
	path := filepath.Join(t.TempDir(), "capture.jsonl")
	recorder, err := cassette.NewRecorder(path, testHeader(upstream.URL), false, false)
	if err != nil {
		t.Fatal(err)
	}
	recorderServer := httptest.NewServer(NewRecord(RecordOptions{Target: target, Cassette: recorder, Redactions: cassette.DefaultRedactions}))
	response, err := http.Post(recorderServer.URL+"/login?access_token=live-query-token", "application/json", strings.NewReader(`{"password":"live-password"}`))
	if err != nil {
		t.Fatal(err)
	}
	liveBody, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || !strings.Contains(string(liveBody), "live-server-token") {
		t.Fatalf("live response was changed: %d %s", response.StatusCode, liveBody)
	}
	_ = recorder.Close()
	recorderServer.Close()
	upstream.Close()
	header, entries, err := cassette.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if upstreamPassword != "live-password" {
		t.Fatalf("upstream got password %q", upstreamPassword)
	}
	stored, _ := json.Marshal(entries[0])
	if strings.Contains(string(stored), "live-password") || strings.Contains(string(stored), "live-query-token") || strings.Contains(string(stored), "live-server-token") {
		t.Fatalf("cassette contains a secret: %s", stored)
	}
	replay := httptest.NewServer(NewReplay(header, entries, false, nil, 0))
	defer replay.Close()
	request, _ := http.NewRequest("POST", replay.URL+"/login?access_token=another-token", strings.NewReader(`{"password":"another-password"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer different")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	replayedBody, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || !strings.Contains(string(replayedBody), cassette.Redacted) || strings.Contains(string(replayedBody), "live-server-token") {
		t.Fatalf("unexpected replay: %d %s", response.StatusCode, replayedBody)
	}
	if response.Header.Get("X-Equal-Replay") != "1" {
		t.Fatal("replay provenance header is missing")
	}
	if upstreamAuth != "" {
		// The recording request above did not set auth; this also guards against accidental global state.
		t.Fatalf("unexpected upstream auth %q", upstreamAuth)
	}
}

func TestRecordAndReplayBinary(t *testing.T) {
	payload := []byte{0, 0xff, 1, 2, 3, 0}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(payload)
	}))
	target, _ := url.Parse(upstream.URL)
	path := filepath.Join(t.TempDir(), "binary.jsonl")
	recorder, _ := cassette.NewRecorder(path, testHeader(upstream.URL), false, false)
	server := httptest.NewServer(NewRecord(RecordOptions{Target: target, Cassette: recorder, Redactions: nil}))
	response, err := http.Get(server.URL + "/asset")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if !bytes.Equal(body, payload) {
		t.Fatalf("record changed bytes: %v", body)
	}
	_ = recorder.Close()
	server.Close()
	upstream.Close()
	header, entries, err := cassette.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	replay := httptest.NewServer(NewReplay(header, entries, false, nil, 0))
	defer replay.Close()
	response, err = http.Get(replay.URL + "/asset")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(response.Body)
	response.Body.Close()
	if !bytes.Equal(body, payload) {
		t.Fatalf("offline replay changed bytes: %v", body)
	}
}

func TestReplaySequenceAndStrictOrder(t *testing.T) {
	header := testHeader("")
	entries := []cassette.Entry{
		{ID: 1, Request: cassette.Request{Method: "GET", URI: "/same"}, Response: cassette.Response{Status: 500, Body: "first", BodyEncoding: "utf8"}},
		{ID: 2, Request: cassette.Request{Method: "GET", URI: "/same"}, Response: cassette.Response{Status: 200, Body: "second", BodyEncoding: "utf8"}},
		{ID: 3, Request: cassette.Request{Method: "GET", URI: "/other"}, Response: cassette.Response{Status: 200, Body: "other", BodyEncoding: "utf8"}},
	}
	replay := httptest.NewServer(NewReplay(header, entries, false, nil, 0))
	defer replay.Close()
	for index, status := range []int{500, 200, 404} {
		response, err := http.Get(replay.URL + "/same")
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != status {
			t.Fatalf("call %d status = %d, want %d", index+1, response.StatusCode, status)
		}
	}
	strict := httptest.NewServer(NewReplay(header, entries, true, nil, 0))
	defer strict.Close()
	response, err := http.Get(strict.URL + "/other")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("out-of-order request returned %d", response.StatusCode)
	}
	response, err = http.Get(strict.URL + "/same")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 500 {
		t.Fatalf("failed request did not advance sequence: %d", response.StatusCode)
	}
}

func TestParallelReplay(t *testing.T) {
	header := testHeader("")
	entries := make([]cassette.Entry, 100)
	for index := range entries {
		uri := fmt.Sprintf("/item/%d", index)
		entries[index] = cassette.Entry{ID: uint64(index + 1), Request: cassette.Request{Method: "GET", URI: uri}, Response: cassette.Response{Status: 200, Body: uri, BodyEncoding: "utf8"}}
	}
	replay := httptest.NewServer(NewReplay(header, entries, false, nil, 0))
	defer replay.Close()
	var failures atomic.Int32
	var wait sync.WaitGroup
	for index := 0; index < 100; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			uri := fmt.Sprintf("/item/%d", index)
			response, err := http.Get(replay.URL + uri)
			if err != nil {
				failures.Add(1)
				return
			}
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode != 200 || string(body) != uri {
				failures.Add(1)
			}
		}(index)
	}
	wait.Wait()
	if failures.Load() != 0 {
		t.Fatalf("%d parallel replays failed", failures.Load())
	}
}

func TestVerifyCompatibleDriftAndInjectedCredentials(t *testing.T) {
	header := testHeader("")
	entries := []cassette.Entry{{
		ID:       1,
		Request:  cassette.Request{Method: "GET", URI: "/items?a=1", Headers: http.Header{"Authorization": {cassette.Redacted}}},
		Response: cassette.Response{Status: 200, Headers: http.Header{"Content-Type": {"application/json; charset=UTF-8"}}, Body: `{"id":"original","value":1,"nested":{"ok":true}}`, BodyEncoding: "utf8"},
	}}
	candidate := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer candidate" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"nested":{"ok":true},"value":1.0,"id":"candidate"}`))
	}))
	defer candidate.Close()
	target, _ := url.Parse(candidate.URL)
	report, err := Verify(header, entries, VerifyOptions{Target: target, Headers: http.Header{"Authorization": {"Bearer candidate"}}, IgnoreJSON: []string{"/id"}})
	if err != nil || report.Failed != 0 {
		t.Fatalf("compatible candidate failed: %+v, %v", report, err)
	}
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"candidate","value":2,"nested":{"ok":false}}`))
	}))
	defer broken.Close()
	target, _ = url.Parse(broken.URL)
	report, err = Verify(header, entries, VerifyOptions{Target: target, IgnoreJSON: []string{"/id"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Failed != 1 || len(report.Results[0].Differences) < 2 || report.Error() == nil {
		t.Fatalf("incompatible candidate was not explained: %+v", report)
	}
}
