package lab

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/Balanov/equal/internal/cassette"
)

type RecordOptions struct {
	Target     *url.URL
	Cassette   *cassette.Recorder
	Redactions []string
	Client     *http.Client
	MaxBody    int64
}

type RecordHandler struct {
	target     *url.URL
	cassette   *cassette.Recorder
	redactions []string
	client     *http.Client
	maxBody    int64
	next       atomic.Uint64
}

func NewRecord(options RecordOptions) *RecordHandler {
	if options.Client == nil {
		options.Client = HTTPClient(30 * time.Second)
	}
	if options.MaxBody <= 0 {
		options.MaxBody = DefaultMaxBody
	}
	return &RecordHandler{target: options.Target, cassette: options.Cassette, redactions: options.Redactions, client: options.Client, maxBody: options.MaxBody}
}

func (h *RecordHandler) ServeHTTP(w http.ResponseWriter, incoming *http.Request) {
	started := time.Now()
	id := h.next.Add(1)
	requestBody, err := readBody(incoming.Body, h.maxBody)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "request_body", err)
		return
	}
	upstream, err := outgoingRequest(incoming, h.target, requestBody)
	if err != nil {
		writeError(w, http.StatusBadRequest, "request", err)
		return
	}
	if clientIP := clientIP(incoming); clientIP != "" {
		upstream.Header.Add("X-Forwarded-For", clientIP)
	}
	response, err := h.client.Do(upstream)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream", err)
		return
	}
	responseBody, err := readBody(response.Body, h.maxBody)
	if err != nil {
		writeError(w, http.StatusBadGateway, "response_body", err)
		return
	}
	storedRequest := recordedRequest(incoming, requestBody)
	cassette.RedactRequest(&storedRequest, h.redactions)
	liveResponse := recordedResponse(response.StatusCode, response.Header, responseBody)
	storedResponse := recordedResponse(response.StatusCode, response.Header, responseBody)
	cassette.RedactResponse(&storedResponse, h.redactions)
	entry := cassette.Entry{ID: id, Request: storedRequest, Response: storedResponse, DurationMS: time.Since(started).Milliseconds()}
	if err := h.cassette.Write(entry); err != nil {
		writeError(w, http.StatusInternalServerError, "cassette", fmt.Errorf("record interaction %d: %w", id, err))
		return
	}
	writeResponse(w, liveResponse, "X-Equal-Record", id)
}

func Listen(address string) (net.Listener, error) {
	return net.Listen("tcp", address)
}
