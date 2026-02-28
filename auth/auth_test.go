package auth

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestPromptCredentials(t *testing.T) {
	orig := readPassword
	t.Cleanup(func() { readPassword = orig })
	readPassword = func(fd int) ([]byte, error) { return []byte("secret"), nil }

	in := strings.NewReader("alice\n")
	var out bytes.Buffer

	username, password, err := PromptCredentials(in, &out)
	if err != nil {
		t.Fatalf("PromptCredentials failed: %v", err)
	}
	if username != "alice" {
		t.Fatalf("username = %q, want %q", username, "alice")
	}
	if password != "secret" {
		t.Fatalf("password = %q, want %q", password, "secret")
	}
	if got := out.String(); got != "login: password: \n" {
		t.Fatalf("prompt output = %q", got)
	}
}

func TestPromptCredentialsPasswordError(t *testing.T) {
	orig := readPassword
	t.Cleanup(func() { readPassword = orig })
	readPassword = func(fd int) ([]byte, error) { return nil, errors.New("boom") }

	_, _, err := PromptCredentials(strings.NewReader("alice\n"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error")
	}
}
