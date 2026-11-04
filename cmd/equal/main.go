package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Balanov/equal/internal/cassette"
	"github.com/Balanov/equal/internal/lab"
)

var Version = "0.1.0"

type stringsFlag []string

func (s *stringsFlag) String() string { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		fmt.Fprintf(stdout, "EQUAL %s\n", Version)
		return 0
	}
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	var err error
	incompatible := false
	switch args[0] {
	case "record":
		err = runRecord(ctx, args[1:], stderr)
	case "replay":
		err = runReplay(ctx, args[1:], stderr)
	case "verify":
		incompatible, err = runVerify(args[1:], stdout)
	case "inspect":
		err = runInspect(args[1:], stdout)
	case "help", "--help", "-h":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
	if err == nil {
		return 0
	}
	if incompatible {
		return 3
	}
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintln(stderr, "equal:", err)
	return 1
}

func usage(writer io.Writer) {
	fmt.Fprintln(writer, `EQUAL — one cassette, both sides of the API.

Usage:
  equal record --target URL --cassette FILE [--listen ADDR]
  equal replay --cassette FILE [--listen ADDR] [--strict]
  equal verify --target URL --cassette FILE [--json REPORT]
  equal inspect --cassette FILE [--json]
  equal --version

Defaults: listen 127.0.0.1:8080, max body 16 MiB.
Exit codes: 0 compatible, 1 runtime error, 2 usage, 3 incompatible.`)
}

func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags
}

func runRecord(ctx context.Context, args []string, stderr io.Writer) error {
	flags := newFlags("record", stderr)
	listen := flags.String("listen", "127.0.0.1:8080", "listen address")
	targetRaw := flags.String("target", "", "upstream base URL")
	path := flags.String("cassette", "", "output JSONL cassette")
	maxBody := flags.Int64("max-body", lab.DefaultMaxBody, "maximum body size in bytes")
	timeout := flags.Duration("timeout", 30*time.Second, "upstream request timeout")
	overwrite := flags.Bool("overwrite", false, "replace an existing cassette")
	fsync := flags.Bool("fsync", false, "fsync every recorded interaction")
	noRedact := flags.Bool("no-redact", false, "disable built-in secret redaction")
	var extraRedactions stringsFlag
	flags.Var(&extraRedactions, "redact", "additional header, query, or JSON field name")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *targetRaw == "" || *path == "" {
		return errors.New("record requires --target and --cassette and accepts no positional arguments")
	}
	if *maxBody <= 0 || *timeout <= 0 {
		return errors.New("--max-body and --timeout must be positive")
	}
	target, err := lab.ParseTarget(*targetRaw)
	if err != nil {
		return err
	}
	redactions := []string{}
	if !*noRedact {
		redactions = append(redactions, cassette.DefaultRedactions...)
	}
	redactions = cassette.NormalizedNames(append(redactions, extraRedactions...))
	header := cassette.Header{Format: cassette.Format, CreatedAt: time.Now().UTC().Format(time.RFC3339), Target: target.String(), Redactions: redactions}
	recorder, err := cassette.NewRecorder(*path, header, *overwrite, *fsync)
	if err != nil {
		return err
	}
	defer recorder.Close()
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "EQUAL record %s → %s\nlistening on http://%s\ncassette %s\n", listener.Addr(), target, listener.Addr(), *path)
	handler := lab.NewRecord(lab.RecordOptions{Target: target, Cassette: recorder, Redactions: redactions, Client: lab.HTTPClient(*timeout), MaxBody: *maxBody})
	return lab.Serve(ctx, listener, handler)
}

func runReplay(ctx context.Context, args []string, stderr io.Writer) error {
	flags := newFlags("replay", stderr)
	listen := flags.String("listen", "127.0.0.1:8080", "listen address")
	path := flags.String("cassette", "", "input JSONL cassette")
	strict := flags.Bool("strict", false, "require the exact global interaction order")
	maxBody := flags.Int64("max-body", lab.DefaultMaxBody, "maximum body size in bytes")
	var matchHeaders stringsFlag
	flags.Var(&matchHeaders, "match-header", "request header included in matching")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *path == "" {
		return errors.New("replay requires --cassette and accepts no positional arguments")
	}
	if *maxBody <= 0 {
		return errors.New("--max-body must be positive")
	}
	header, entries, err := cassette.Load(*path)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "EQUAL replay %d interactions from %s\nlistening on http://%s\nupstream network: disabled\n", len(entries), *path, listener.Addr())
	handler := lab.NewReplay(header, entries, *strict, cassette.NormalizedNames(matchHeaders), *maxBody)
	return lab.Serve(ctx, listener, handler)
}

func runVerify(args []string, stdout io.Writer) (bool, error) {
	flags := newFlags("verify", stdout)
	targetRaw := flags.String("target", "", "candidate base URL")
	path := flags.String("cassette", "", "input JSONL cassette")
	reportPath := flags.String("json", "", "write a machine-readable JSON report")
	maxBody := flags.Int64("max-body", lab.DefaultMaxBody, "maximum body size in bytes")
	timeout := flags.Duration("timeout", 30*time.Second, "candidate request timeout")
	var ignoreJSON, compareHeaders, rawHeaders stringsFlag
	flags.Var(&ignoreJSON, "ignore-json", "JSON Pointer to ignore; * matches one segment")
	flags.Var(&compareHeaders, "compare-header", "response header to compare; repeatable")
	flags.Var(&rawHeaders, "header", "candidate request header 'Name: value'")
	if err := flags.Parse(args); err != nil {
		return false, err
	}
	if flags.NArg() != 0 || *targetRaw == "" || *path == "" {
		return false, errors.New("verify requires --target and --cassette and accepts no positional arguments")
	}
	if *maxBody <= 0 || *timeout <= 0 {
		return false, errors.New("--max-body and --timeout must be positive")
	}
	target, err := lab.ParseTarget(*targetRaw)
	if err != nil {
		return false, err
	}
	headers, err := parseHeaders(rawHeaders)
	if err != nil {
		return false, err
	}
	header, entries, err := cassette.Load(*path)
	if err != nil {
		return false, err
	}
	report, err := lab.Verify(header, entries, lab.VerifyOptions{
		Target: target, Client: lab.HTTPClient(*timeout), MaxBody: *maxBody,
		Headers: headers, IgnoreJSON: ignoreJSON, CompareHeaders: compareHeaders,
	})
	if err != nil {
		return false, err
	}
	for _, result := range report.Results {
		for _, difference := range result.Differences {
			fmt.Fprintf(stdout, "FAIL #%d %s %s\n  %s\n  expected: %s\n  actual:   %s\n", result.ID, result.Method, result.URI, difference.Field, difference.Expected, difference.Actual)
		}
	}
	fmt.Fprintf(stdout, "EQUAL verify: %d/%d compatible · %d failed\n", report.Passed, report.Total, report.Failed)
	if *reportPath != "" {
		file, err := os.OpenFile(*reportPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return false, err
		}
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		err = encoder.Encode(report)
		closeErr := file.Close()
		if err != nil {
			return false, err
		}
		if closeErr != nil {
			return false, closeErr
		}
	}
	return report.Failed > 0, report.Error()
}

func runInspect(args []string, stdout io.Writer) error {
	flags := newFlags("inspect", stdout)
	path := flags.String("cassette", "", "input JSONL cassette")
	asJSON := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *path == "" {
		return errors.New("inspect requires --cassette and accepts no positional arguments")
	}
	header, entries, err := cassette.Load(*path)
	if err != nil {
		return err
	}
	if *asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]any{"header": header, "interactions": len(entries)})
	}
	fmt.Fprintf(stdout, "format:       %s\ncreated_at:   %s\ntarget:       %s\ninteractions: %d\nredactions:   %s\n", header.Format, header.CreatedAt, header.Target, len(entries), strings.Join(header.Redactions, ", "))
	for _, entry := range entries {
		fmt.Fprintf(stdout, "#%d %s %s → %d\n", entry.ID, entry.Request.Method, entry.Request.URI, entry.Response.Status)
	}
	return nil
}

func parseHeaders(values []string) (http.Header, error) {
	headers := make(http.Header)
	for _, value := range values {
		name, content, ok := strings.Cut(value, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("invalid --header %q; expected 'Name: value'", value)
		}
		headers.Add(name, strings.TrimSpace(content))
	}
	return headers, nil
}
