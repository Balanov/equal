package cassette

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
)

type Recorder struct {
	mu     sync.Mutex
	file   *os.File
	enc    *json.Encoder
	fsync  bool
	closed bool
}

func NewRecorder(path string, header Header, overwrite, fsync bool) (*Recorder, error) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_EXCL
	if overwrite {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return nil, err
	}
	recorder := &Recorder{file: file, enc: json.NewEncoder(file), fsync: fsync}
	recorder.enc.SetEscapeHTML(false)
	if err := recorder.enc.Encode(header); err != nil {
		file.Close()
		return nil, err
	}
	return recorder, nil
}

func (r *Recorder) Write(entry Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return errors.New("cassette is closed")
	}
	if err := r.enc.Encode(entry); err != nil {
		return err
	}
	if r.fsync {
		return r.file.Sync()
	}
	return nil
}

func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	return r.file.Close()
}

func Load(path string) (Header, []Entry, error) {
	file, err := os.Open(path)
	if err != nil {
		return Header{}, nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	decoder := json.NewDecoder(reader)
	var header Header
	if err := decoder.Decode(&header); err != nil {
		return Header{}, nil, fmt.Errorf("read cassette header: %w", err)
	}
	if header.Format != Format {
		return Header{}, nil, fmt.Errorf("unsupported cassette format %q", header.Format)
	}
	entries := []Entry{}
	seen := map[uint64]bool{}
	for {
		var entry Entry
		err := decoder.Decode(&entry)
		if err == io.EOF {
			break
		}
		if err != nil {
			return Header{}, nil, fmt.Errorf("read interaction %d: %w", len(entries)+1, err)
		}
		if entry.ID == 0 || seen[entry.ID] {
			return Header{}, nil, fmt.Errorf("invalid or duplicate interaction id %d", entry.ID)
		}
		if entry.Request.Method == "" || entry.Request.URI == "" || entry.Response.Status < 100 || entry.Response.Status > 999 {
			return Header{}, nil, fmt.Errorf("invalid interaction %d", entry.ID)
		}
		if _, err := entry.Request.Bytes(); err != nil {
			return Header{}, nil, fmt.Errorf("interaction %d request: %w", entry.ID, err)
		}
		if _, err := entry.Response.Bytes(); err != nil {
			return Header{}, nil, fmt.Errorf("interaction %d response: %w", entry.ID, err)
		}
		seen[entry.ID] = true
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return Header{}, nil, errors.New("cassette has no interactions")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return header, entries, nil
}
