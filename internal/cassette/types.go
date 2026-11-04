package cassette

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"unicode/utf8"
)

const (
	Format   = "equal/v1"
	Redacted = "[REDACTED]"
)

type Header struct {
	Format     string   `json:"format"`
	CreatedAt  string   `json:"created_at"`
	Target     string   `json:"target"`
	Redactions []string `json:"redactions,omitempty"`
}

type Request struct {
	Method       string      `json:"method"`
	URI          string      `json:"uri"`
	Headers      http.Header `json:"headers,omitempty"`
	Body         string      `json:"body,omitempty"`
	BodyEncoding string      `json:"body_encoding,omitempty"`
}

type Response struct {
	Status       int         `json:"status"`
	Headers      http.Header `json:"headers,omitempty"`
	Body         string      `json:"body,omitempty"`
	BodyEncoding string      `json:"body_encoding,omitempty"`
}

type Entry struct {
	ID         uint64   `json:"id"`
	Request    Request  `json:"request"`
	Response   Response `json:"response"`
	DurationMS int64    `json:"duration_ms,omitempty"`
}

func SetBody(data []byte) (string, string) {
	if len(data) == 0 {
		return "", ""
	}
	if utf8.Valid(data) {
		return string(data), "utf8"
	}
	return base64.StdEncoding.EncodeToString(data), "base64"
}

func BodyBytes(body, encoding string) ([]byte, error) {
	switch encoding {
	case "", "utf8":
		return []byte(body), nil
	case "base64":
		data, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 body: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unknown body encoding %q", encoding)
	}
}

func (r Request) Bytes() ([]byte, error)  { return BodyBytes(r.Body, r.BodyEncoding) }
func (r Response) Bytes() ([]byte, error) { return BodyBytes(r.Body, r.BodyEncoding) }
