package auth

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Authenticator validates user credentials.
type Authenticator interface {
	Authenticate(username, password string) error
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
