package lab

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Balanov/equal/internal/cassette"
)

type VerifyOptions struct {
	Target         *url.URL
	Client         *http.Client
	MaxBody        int64
	Headers        http.Header
	IgnoreJSON     []string
	CompareHeaders []string
}

type VerifyResult struct {
	ID          uint64                `json:"id"`
	Method      string                `json:"method"`
	URI         string                `json:"uri"`
	DurationMS  int64                 `json:"duration_ms"`
	Passed      bool                  `json:"passed"`
	Differences []cassette.Difference `json:"differences,omitempty"`
}

type VerifyReport struct {
	Format  string         `json:"format"`
	Target  string         `json:"target"`
	Total   int            `json:"total"`
	Passed  int            `json:"passed"`
	Failed  int            `json:"failed"`
	Results []VerifyResult `json:"results"`
}

func Verify(header cassette.Header, entries []cassette.Entry, options VerifyOptions) (VerifyReport, error) {
	if options.Client == nil {
		options.Client = HTTPClient(30 * time.Second)
	}
	if options.MaxBody <= 0 {
		options.MaxBody = DefaultMaxBody
	}
	if options.CompareHeaders == nil {
		options.CompareHeaders = []string{"Content-Type", "Location"}
	}
	report := VerifyReport{Format: cassette.Format, Target: options.Target.String(), Total: len(entries), Results: make([]VerifyResult, 0, len(entries))}
	for _, entry := range entries {
		started := time.Now()
		result := VerifyResult{ID: entry.ID, Method: entry.Request.Method, URI: entry.Request.URI}
		body, err := entry.Request.Bytes()
		if err == nil {
			var target *url.URL
			target, err = targetURL(options.Target, entry.Request.URI)
			if err == nil {
				var request *http.Request
				request, err = http.NewRequest(entry.Request.Method, target.String(), bytes.NewReader(body))
				if err == nil {
					request.Header = entry.Request.Headers.Clone()
					removeHop(request.Header)
					for name, values := range options.Headers {
						request.Header.Del(name)
						for _, value := range values {
							request.Header.Add(name, value)
						}
					}
					request.Header.Set("Accept-Encoding", "identity")
					var response *http.Response
					response, err = options.Client.Do(request)
					if err == nil {
						var responseBody []byte
						responseBody, err = readBody(response.Body, options.MaxBody)
						if err == nil {
							actual := recordedResponse(response.StatusCode, response.Header, responseBody)
							cassette.RedactResponse(&actual, header.Redactions)
							result.Differences = cassette.Compare(entry.Response, actual, options.IgnoreJSON, options.CompareHeaders)
						}
					}
				}
			}
		}
		if err != nil {
			result.Differences = []cassette.Difference{{Field: "transport", Expected: "completed response", Actual: err.Error()}}
		}
		result.DurationMS = time.Since(started).Milliseconds()
		result.Passed = len(result.Differences) == 0
		if result.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
		report.Results = append(report.Results, result)
	}
	return report, nil
}

func (r VerifyReport) Error() error {
	if r.Failed == 0 {
		return nil
	}
	return fmt.Errorf("%d of %d interactions are incompatible", r.Failed, r.Total)
}
