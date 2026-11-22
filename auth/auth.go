package auth

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// NoticeStyle classifies informational or error messages produced by PAM conversation callbacks.
type NoticeStyle int

const (
	NoticeInfo NoticeStyle = iota
	NoticeError
)

// Notice represents a single informational or error message emitted during PAM stages.
type Notice struct {
	Style NoticeStyle
	Text  string
}

// FlushNotices writes buffered PAM notices to stdout/stderr without altering internal whitespace
// (such as MOTD formatting), skipping empty/whitespace-only entries.
func FlushNotices(notices []Notice, stdout, stderr io.Writer) {
	for _, n := range notices {
		if strings.TrimSpace(n.Text) == "" {
			continue
		}
		out := stdout
		if n.Style == NoticeError {
			out = stderr
		}
		if out == nil {
			continue
		}
		if strings.HasSuffix(n.Text, "\n") {
			_, _ = fmt.Fprint(out, n.Text)
		} else {
			_, _ = fmt.Fprintln(out, n.Text)
		}
	}
}

// LoginSession represents an authenticated PAM transaction lifecycle.
type LoginSession interface {
	Username() string
	SetEnv(key, value string) error
	OpenSession() error
	Env() []string
	DrainNotices() []Notice
	CloseSession() error
}

// Authenticator validates user credentials.
type Authenticator interface {
	Authenticate(username, password string) (LoginSession, error)
}

var readPassword = func(fd int) ([]byte, error) {
	return term.ReadPassword(fd)
}

// PromptCredentials reads username and password from terminal streams.
func PromptCredentials(in io.Reader, out io.Writer) (string, string, error) {
	reader := bufio.NewReader(in)
	if _, err := fmt.Fprint(out, "login: "); err != nil {
		return "", "", err
	}
	usernameRaw, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	username := strings.TrimSpace(usernameRaw)
	if username == "" {
		return "", "", fmt.Errorf("username cannot be empty")
	}

	if _, err := fmt.Fprint(out, "password: "); err != nil {
		return "", "", err
	}
	passwordBytes, err := readPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", "", err
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return "", "", err
	}
	return username, string(passwordBytes), nil
}
