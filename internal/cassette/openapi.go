package cassette

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var uuidSegment = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type observedOperation struct {
	method  string
	entries []Entry
}

// OpenAPI builds an OpenAPI 3.1 description of observed traffic. It describes
// evidence, not complete provider coverage or a normative provider schema.
func OpenAPI(header Header, entries []Entry, title, version string) map[string]any {
	if title == "" {
		title = "Observed API"
	}
	if version == "" {
		version = "0.1.0"
	}
	groups := map[string]map[string][]Entry{}
	for _, entry := range entries {
		path, _ := observedPath(entry.Request.URI)
		method := strings.ToLower(entry.Request.Method)
		if groups[path] == nil {
			groups[path] = map[string][]Entry{}
		}
		groups[path][method] = append(groups[path][method], entry)
	}
	paths := map[string]any{}
	for path, operations := range groups {
		operationsObject := map[string]any{}
		for method, observed := range operations {
			operationsObject[method] = operationObject(path, method, observed)
		}
		paths[path] = operationsObject
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title": title, "version": version,
			"description": "Generated from observed EQUAL traffic. This is an incomplete, example-driven description, not the provider's authoritative schema.",
		},
		"servers":         []any{map[string]any{"url": header.Target}},
		"paths":           paths,
		"x-equal-format":  Format,
		"x-equal-warning": "Only recorded interactions are represented. Numeric and UUID path segments are inferred parameters; redacted examples remain redacted.",
	}
}

func operationObject(path, method string, entries []Entry) map[string]any {
	ids := make([]uint64, len(entries))
	parameters := observedParameters(path, entries)
	var requestBody any
	for index, entry := range entries {
		ids[index] = entry.ID
	}
	responses := map[string]any{}
	for _, entry := range entries {
		status := strconv.Itoa(entry.Response.Status)
		responses[status] = appendResponse(responses[status], entry)
	}
	for _, entry := range entries {
		body, _ := entry.Request.Bytes()
		if len(body) == 0 || method == "get" || method == "head" {
			continue
		}
		if value, ok := decodeObservedJSON(body); ok {
			requestBody = map[string]any{
				"required": false,
				"content": map[string]any{
					mediaType(entry.Request.Headers.Get("Content-Type"), "application/json"): map[string]any{
						"schema": inferSchema(value), "example": value,
					},
				},
			}
			break
		}
	}
	operation := map[string]any{
		"operationId":                   operationID(path, method),
		"description":                   "Generated from recorded traffic; unobserved behavior is not described.",
		"responses":                     responses,
		"x-equal-observed-interactions": ids,
	}
	if len(parameters) > 0 {
		operation["parameters"] = parameters
	}
	if requestBody != nil {
		operation["requestBody"] = requestBody
	}
	return operation
}

func appendResponse(existing any, entry Entry) any {
	body, _ := entry.Response.Bytes()
	response := map[string]any{"description": "Observed response"}
	if len(body) > 0 {
		contentType := mediaType(entry.Response.Headers.Get("Content-Type"), "application/octet-stream")
		if value, ok := decodeObservedJSON(body); ok {
			content := map[string]any{"schema": inferSchema(value), "example": value}
			response["content"] = map[string]any{contentType: content}
		} else {
			content := map[string]any{"schema": map[string]any{"type": "string"}}
			if entry.Response.BodyEncoding == "base64" {
				content["schema"] = map[string]any{"type": "string", "contentEncoding": "base64"}
				content["example"] = entry.Response.Body
			} else {
				content["example"] = entry.Response.Body
			}
			response["content"] = map[string]any{contentType: content}
		}
	}
	if existing == nil {
		return response
	}
	// Keep the first response in the standard OpenAPI shape and retain every
	// later observation in a provenance extension instead of discarding it.
	previous := existing.(map[string]any)
	alternatives, _ := previous["x-equal-observed-alternatives"].([]any)
	previous["x-equal-observed-alternatives"] = append(alternatives, response)
	return previous
}

func observedParameters(path string, entries []Entry) []any {
	parameters := []any{}
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for _, segment := range segments {
		if strings.HasPrefix(segment, "{") {
			name := strings.Trim(segment, "{}")
			parameters = append(parameters, map[string]any{
				"name": name, "in": "path", "required": true,
				"schema": map[string]any{"type": "string"},
			})
		}
	}
	counts := map[string]int{}
	values := map[string]string{}
	for _, entry := range entries {
		_, rawQuery := observedPath(entry.Request.URI)
		query, _ := url.ParseQuery(rawQuery)
		for key, value := range query {
			counts[key]++
			if len(value) > 0 {
				values[key] = value[0]
			}
		}
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parameters = append(parameters, map[string]any{
			"name": key, "in": "query", "required": counts[key] == len(entries),
			"schema": map[string]any{"type": "string"}, "example": values[key],
		})
	}
	return parameters
}

func observedPath(uri string) (string, string) {
	parsed, err := url.ParseRequestURI(uri)
	if err != nil {
		return uri, ""
	}
	segments := strings.Split(parsed.EscapedPath(), "/")
	parameter := 0
	for index, segment := range segments {
		if segment == "" {
			continue
		}
		if _, err := strconv.Atoi(segment); err == nil || uuidSegment.MatchString(segment) {
			parameter++
			name := "id"
			if parameter > 1 {
				name += strconv.Itoa(parameter)
			}
			segments[index] = "{" + name + "}"
		}
	}
	path := strings.Join(segments, "/")
	if path == "" {
		path = "/"
	}
	return path, parsed.RawQuery
}

func operationID(path, method string) string {
	replacer := strings.NewReplacer("{", "", "}", "", "/", "_", "-", "_")
	return method + replacer.Replace(path)
}

func mediaType(value, fallback string) string {
	parsed, _, err := mime.ParseMediaType(value)
	if err != nil || parsed == "" {
		return fallback
	}
	return parsed
}

func decodeObservedJSON(data []byte) (any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, false
	}
	return value, true
}

func inferSchema(value any) map[string]any {
	switch typed := value.(type) {
	case nil:
		return map[string]any{}
	case bool:
		return map[string]any{"type": "boolean"}
	case json.Number:
		if strings.ContainsAny(typed.String(), ".eE") {
			return map[string]any{"type": "number"}
		}
		return map[string]any{"type": "integer"}
	case string:
		return map[string]any{"type": "string"}
	case []any:
		schema := map[string]any{"type": "array", "items": map[string]any{}}
		if len(typed) > 0 {
			schema["items"] = inferSchema(typed[0])
		}
		return schema
	case map[string]any:
		properties := map[string]any{}
		required := make([]string, 0, len(typed))
		for key, child := range typed {
			properties[key] = inferSchema(child)
			required = append(required, key)
		}
		sort.Strings(required)
		return map[string]any{"type": "object", "properties": properties, "required": required}
	default:
		panic(fmt.Sprintf("unhandled JSON value %T", value))
	}
}
