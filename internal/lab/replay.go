package lab

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/Balanov/equal/internal/cassette"
)

type replayQueue struct {
	entries []cassette.Entry
	next    int
}

type ReplayHandler struct {
	header       cassette.Header
	entries      []cassette.Entry
	strict       bool
	matchHeaders []string
	maxBody      int64
	mu           sync.Mutex
	byKey        map[string]*replayQueue
	nextStrict   int
}

func NewReplay(header cassette.Header, entries []cassette.Entry, strict bool, matchHeaders []string, maxBody int64) *ReplayHandler {
	if maxBody <= 0 {
		maxBody = DefaultMaxBody
	}
	handler := &ReplayHandler{header: header, entries: entries, strict: strict, matchHeaders: matchHeaders, maxBody: maxBody, byKey: make(map[string]*replayQueue)}
	for _, entry := range entries {
		key := cassette.Key(entry.Request, matchHeaders)
		queue := handler.byKey[key]
		if queue == nil {
			queue = &replayQueue{}
			handler.byKey[key] = queue
		}
		queue.entries = append(queue.entries, entry)
	}
	return handler
}

func (h *ReplayHandler) ServeHTTP(w http.ResponseWriter, incoming *http.Request) {
	body, err := readBody(incoming.Body, h.maxBody)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "request_body", err)
		return
	}
	request := recordedRequest(incoming, body)
	cassette.RedactRequest(&request, h.header.Redactions)
	key := cassette.Key(request, h.matchHeaders)
	h.mu.Lock()
	var entry cassette.Entry
	if h.strict {
		if h.nextStrict >= len(h.entries) {
			h.mu.Unlock()
			writeError(w, http.StatusNotFound, "cassette_exhausted", errors.New("all interactions have been replayed"))
			return
		}
		entry = h.entries[h.nextStrict]
		expected := cassette.Key(entry.Request, h.matchHeaders)
		if key != expected {
			h.mu.Unlock()
			writeError(w, http.StatusConflict, "sequence_mismatch", fmt.Errorf("expected interaction %d: %s %s", entry.ID, entry.Request.Method, entry.Request.URI))
			return
		}
		h.nextStrict++
	} else {
		queue := h.byKey[key]
		if queue == nil || queue.next >= len(queue.entries) {
			h.mu.Unlock()
			writeError(w, http.StatusNotFound, "no_matching_interaction", fmt.Errorf("no recorded response for %s %s", request.Method, request.URI))
			return
		}
		entry = queue.entries[queue.next]
		queue.next++
	}
	h.mu.Unlock()
	writeResponse(w, entry.Response, "X-Equal-Replay", entry.ID)
}
