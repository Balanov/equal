package cassette

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"sort"
	"strings"
)

var DefaultRedactions = []string{
	"access_token", "api_key", "apikey", "authorization", "cookie", "id_token",
	"key", "passwd", "password", "proxy-authorization", "refresh_token", "secret",
	"session", "session_id", "set-cookie", "sig", "signature", "token",
	"x-access-token", "x-api-key", "x-auth-token",
}

func nameSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			set[name] = true
		}
	}
	return set
}

func NormalizedNames(names []string) []string {
	set := nameSet(names)
	result := make([]string, 0, len(set))
	for name := range set {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func RedactRequest(request *Request, names []string) {
	redactHeaders(request.Headers, names)
	set := nameSet(names)
	if parsed, err := url.ParseRequestURI(request.URI); err == nil {
		query := parsed.Query()
		changed := false
		for key := range query {
			if set[strings.ToLower(key)] {
				query.Set(key, Redacted)
				changed = true
			}
		}
		if changed {
			parsed.RawQuery = query.Encode()
			request.URI = parsed.RequestURI()
		}
	}
	request.Body, request.BodyEncoding = redactBody(request.Body, request.BodyEncoding, set)
}

func RedactResponse(response *Response, names []string) {
	redactHeaders(response.Headers, names)
	response.Body, response.BodyEncoding = redactBody(response.Body, response.BodyEncoding, nameSet(names))
}

func redactHeaders(headers map[string][]string, names []string) {
	set := nameSet(names)
	for key := range headers {
		if set[strings.ToLower(key)] {
			headers[key] = []string{Redacted}
		}
	}
}

func redactBody(body, encoding string, names map[string]bool) (string, string) {
	if encoding != "utf8" || body == "" || len(names) == 0 {
		return body, encoding
	}
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return body, encoding
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return body, encoding
	}
	changed := redactJSON(value, names)
	if !changed {
		return body, encoding
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return body, encoding
	}
	return strings.TrimSuffix(out.String(), "\n"), "utf8"
}

func redactJSON(value any, names map[string]bool) bool {
	changed := false
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if names[strings.ToLower(key)] {
				typed[key] = Redacted
				changed = true
			} else if redactJSON(child, names) {
				changed = true
			}
		}
	case []any:
		for _, child := range typed {
			if redactJSON(child, names) {
				changed = true
			}
		}
	}
	return changed
}
