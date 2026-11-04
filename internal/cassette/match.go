package cassette

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"strings"
)

type match struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   string              `json:"query"`
	Body    string              `json:"body"`
	Headers map[string][]string `json:"headers,omitempty"`
}

func Key(request Request, matchHeaders []string) string {
	path := request.URI
	query := ""
	if parsed, err := url.ParseRequestURI(request.URI); err == nil {
		path = parsed.EscapedPath()
		if path == "" {
			path = "/"
		}
		query = parsed.Query().Encode()
	}
	body, bodyEncoding := request.Body, request.BodyEncoding
	if bodyEncoding == "utf8" || bodyEncoding == "" {
		body = CanonicalJSON([]byte(body))
	}
	selected := make(map[string][]string, len(matchHeaders))
	for _, name := range matchHeaders {
		values, ok := request.Headers[httpCanonical(name)]
		if ok {
			selected[strings.ToLower(name)] = values
		}
	}
	encoded, _ := json.Marshal(match{
		Method: strings.ToUpper(request.Method), Path: path, Query: query,
		Body: bodyEncoding + ":" + body, Headers: selected,
	})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func CanonicalJSON(data []byte) string {
	if len(bytes.TrimSpace(data)) == 0 {
		return ""
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return string(data)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return string(data)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return string(data)
	}
	return string(encoded)
}

func httpCanonical(value string) string {
	return textprotoCanonicalMIMEHeaderKey(value)
}
