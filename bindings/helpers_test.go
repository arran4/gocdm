package bindings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigJSON(t *testing.T) {
	var got response
	if err := json.Unmarshal([]byte(DefaultConfigJSON()), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !got.OK {
		t.Fatalf("expected ok response, got: %+v", got)
	}
	if len(got.Data) == 0 {
		t.Fatal("expected config payload data")
	}
}

func TestMarshalErrorResponse(t *testing.T) {
	gotJSON := marshal(make(chan int), nil)

	var got response
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.OK {
		t.Fatalf("expected error response, got: %+v", got)
	}
	if got.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestDiscoverSessionsJSON(t *testing.T) {
	tmp := t.TempDir()
	xsession := filepath.Join(tmp, ".xsession")
	if err := os.WriteFile(xsession, []byte("#!/bin/sh\nexec xterm\n"), 0o755); err != nil {
		t.Fatalf("write xsession failed: %v", err)
	}

	var got response
	if err := json.Unmarshal([]byte(DiscoverSessionsJSON(tmp)), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !got.OK {
		t.Fatalf("expected ok response, got: %+v", got)
	}

	var sessions []map[string]any
	if err := json.Unmarshal(got.Data, &sessions); err != nil {
		t.Fatalf("invalid data payload: %v", err)
	}
	if len(sessions) < 1 {
		t.Fatalf("expected at least one session, got %d", len(sessions))
	}
}
