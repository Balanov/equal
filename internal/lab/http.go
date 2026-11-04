package lab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Balanov/equal/internal/cassette"
)

const DefaultMaxBody = int64(16 << 20)

func ParseTarget(raw string) (*url.URL, error) {
	target, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		return nil, errors.New("target must be an absolute http:// or https:// URL")
	}
	if target.User != nil || target.RawQuery != "" || target.Fragment != "" {
		return nil, errors.New("target cannot contain credentials, query, or fragment")
	}
	target.Path = strings.TrimSuffix(target.Path, "/")
	return target, nil
}

func HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment, DisableCompression: true,
			MaxIdleConns: 100, MaxIdleConnsPerHost: 100, IdleConnTimeout: 90 * time.Second,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func readBody(body io.ReadCloser, max int64) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	defer body.Close()
	reader := io.LimitReader(body, max+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("body exceeds %d bytes", max)
	}
	return data, nil
}

func targetURL(target *url.URL, uri string) (*url.URL, error) {
	requestURI, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, err
	}
	result := *target
	result.Path = target.Path + requestURI.Path
	if result.Path == "" {
		result.Path = "/"
	}
	result.RawQuery = requestURI.RawQuery
	return &result, nil
}

func outgoingRequest(ctx *http.Request, target *url.URL, body []byte) (*http.Request, error) {
	url, err := targetURL(target, ctx.URL.RequestURI())
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx.Context(), ctx.Method, url.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header = ctx.Header.Clone()
	removeHop(request.Header)
	request.Header.Set("Accept-Encoding", "identity")
	return request, nil
}

func recordedRequest(request *http.Request, body []byte) cassette.Request {
	text, encoding := cassette.SetBody(body)
	return cassette.Request{
		Method: request.Method, URI: request.URL.RequestURI(), Headers: request.Header.Clone(),
		Body: text, BodyEncoding: encoding,
	}
}

func recordedResponse(status int, headers http.Header, body []byte) cassette.Response {
	text, encoding := cassette.SetBody(body)
	return cassette.Response{Status: status, Headers: headers.Clone(), Body: text, BodyEncoding: encoding}
}

func writeResponse(w http.ResponseWriter, response cassette.Response, diagnostic string, value any) {
	headers := response.Headers.Clone()
	removeHop(headers)
	headers.Del("Content-Length")
	copyHeaders(w.Header(), headers)
	if diagnostic != "" {
		w.Header().Set(diagnostic, fmt.Sprint(value))
	}
	w.WriteHeader(response.Status)
	body, _ := response.Bytes()
	if len(body) > 0 && response.Status != http.StatusNoContent && response.Status != http.StatusNotModified {
		_, _ = w.Write(body)
	}
}

func writeError(w http.ResponseWriter, status int, code string, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "detail": err.Error()})
}

func copyHeaders(destination, source http.Header) {
	for key, values := range source {
		for _, value := range values {
			destination.Add(key, value)
		}
	}
}

func removeHop(headers http.Header) {
	for _, value := range headers.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			headers.Del(strings.TrimSpace(name))
		}
	}
	for _, name := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade"} {
		headers.Del(name)
	}
}

func clientIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}
