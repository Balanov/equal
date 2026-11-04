package cassette

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type Difference struct {
	Field    string `json:"field"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

func Compare(expected, actual Response, ignoreJSON []string, compareHeaders []string) []Difference {
	differences := []Difference{}
	if expected.Status != actual.Status {
		differences = append(differences, Difference{Field: "status", Expected: strconv.Itoa(expected.Status), Actual: strconv.Itoa(actual.Status)})
	}
	for _, name := range compareHeaders {
		canonical := http.CanonicalHeaderKey(name)
		want := expected.Headers.Values(canonical)
		got := actual.Headers.Values(canonical)
		if strings.EqualFold(name, "Content-Type") {
			want = normalizeContentTypes(want)
			got = normalizeContentTypes(got)
		}
		if !reflect.DeepEqual(want, got) {
			differences = append(differences, Difference{Field: "header/" + strings.ToLower(name), Expected: render(want), Actual: render(got)})
		}
	}
	expectedBody, expectedErr := expected.Bytes()
	actualBody, actualErr := actual.Bytes()
	if expectedErr != nil || actualErr != nil {
		differences = append(differences, Difference{Field: "body/encoding", Expected: render(expectedErr), Actual: render(actualErr)})
		return differences
	}
	wantValue, wantOK := decodeJSON(expectedBody)
	gotValue, gotOK := decodeJSON(actualBody)
	if wantOK && gotOK {
		for _, path := range ignoreJSON {
			segments := splitPointer(path)
			if len(segments) > 0 {
				wantValue = removePointer(wantValue, segments)
				gotValue = removePointer(gotValue, segments)
			}
		}
		differences = append(differences, diffJSON("", wantValue, gotValue, 100)...)
	} else if !bytes.Equal(expectedBody, actualBody) {
		differences = append(differences, Difference{Field: "body", Expected: short(expectedBody), Actual: short(actualBody)})
	}
	return differences
}

func decodeJSON(data []byte) (any, bool) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, false
	}
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

func diffJSON(path string, expected, actual any, limit int) []Difference {
	if limit <= 0 {
		return nil
	}
	if numbersEqual(expected, actual) {
		return nil
	}
	switch want := expected.(type) {
	case map[string]any:
		got, ok := actual.(map[string]any)
		if !ok {
			return []Difference{{Field: pointer(path), Expected: shortJSON(expected), Actual: shortJSON(actual)}}
		}
		differences := []Difference{}
		keys := make([]string, 0, len(want))
		for key := range want {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := want[key]
			other, exists := got[key]
			if !exists {
				differences = append(differences, Difference{Field: pointer(path + "/" + escape(key)), Expected: shortJSON(value), Actual: "<missing>"})
			} else {
				differences = append(differences, diffJSON(path+"/"+escape(key), value, other, limit-len(differences))...)
			}
			if len(differences) >= limit {
				return differences[:limit]
			}
		}
		for key, value := range got {
			if _, exists := want[key]; !exists {
				differences = append(differences, Difference{Field: pointer(path + "/" + escape(key)), Expected: "<missing>", Actual: shortJSON(value)})
				if len(differences) >= limit {
					return differences[:limit]
				}
			}
		}
		return differences
	case []any:
		got, ok := actual.([]any)
		if !ok {
			return []Difference{{Field: pointer(path), Expected: shortJSON(expected), Actual: shortJSON(actual)}}
		}
		differences := []Difference{}
		common := len(want)
		if len(got) < common {
			common = len(got)
		}
		for index := 0; index < common; index++ {
			differences = append(differences, diffJSON(fmt.Sprintf("%s/%d", path, index), want[index], got[index], limit-len(differences))...)
			if len(differences) >= limit {
				return differences[:limit]
			}
		}
		for index := common; index < len(want); index++ {
			differences = append(differences, Difference{Field: pointer(fmt.Sprintf("%s/%d", path, index)), Expected: shortJSON(want[index]), Actual: "<missing>"})
		}
		for index := common; index < len(got); index++ {
			differences = append(differences, Difference{Field: pointer(fmt.Sprintf("%s/%d", path, index)), Expected: "<missing>", Actual: shortJSON(got[index])})
		}
		if len(differences) > limit {
			return differences[:limit]
		}
		return differences
	default:
		if reflect.DeepEqual(expected, actual) {
			return nil
		}
		return []Difference{{Field: pointer(path), Expected: shortJSON(expected), Actual: shortJSON(actual)}}
	}
}

func numbersEqual(expected, actual any) bool {
	want, wantOK := expected.(json.Number)
	got, gotOK := actual.(json.Number)
	if !wantOK || !gotOK {
		return false
	}
	wantRat, wantOK := new(big.Rat).SetString(want.String())
	gotRat, gotOK := new(big.Rat).SetString(got.String())
	return wantOK && gotOK && wantRat.Cmp(gotRat) == 0
}

func splitPointer(path string) []string {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	for index := range parts {
		parts[index] = strings.ReplaceAll(strings.ReplaceAll(parts[index], "~1", "/"), "~0", "~")
	}
	return parts
}

func removePointer(value any, path []string) any {
	if len(path) == 0 {
		return value
	}
	switch typed := value.(type) {
	case map[string]any:
		key := path[0]
		if len(path) == 1 {
			if key == "*" {
				for child := range typed {
					delete(typed, child)
				}
			} else {
				delete(typed, key)
			}
			return typed
		}
		if key == "*" {
			for childKey, child := range typed {
				typed[childKey] = removePointer(child, path[1:])
			}
		} else if child, ok := typed[key]; ok {
			typed[key] = removePointer(child, path[1:])
		}
	case []any:
		key := path[0]
		for index, child := range typed {
			if key == "*" || key == strconv.Itoa(index) {
				if len(path) == 1 {
					typed[index] = nil
				} else {
					typed[index] = removePointer(child, path[1:])
				}
			}
		}
	}
	return value
}

func normalizeContentTypes(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		mediaType, params, err := mime.ParseMediaType(value)
		if err != nil {
			result[index] = strings.ToLower(strings.TrimSpace(value))
			continue
		}
		result[index] = strings.ToLower(mediaType) + ";" + strings.ToLower(render(params))
	}
	return result
}

func pointer(path string) string {
	if path == "" {
		return "body"
	}
	return "body" + path
}

func escape(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func shortJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return render(value)
	}
	return short(encoded)
}

func short(data []byte) string {
	const limit = 240
	value := string(data)
	if len(value) > limit {
		return value[:limit] + "…"
	}
	return value
}

func render(value any) string {
	return fmt.Sprint(value)
}
