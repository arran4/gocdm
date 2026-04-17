//go:build !nopam

package auth

import (
	"fmt"
	"os"
	"strings"

	"github.com/msteinert/pam/v2"
)

type pamAuthenticator struct {
	Service string
}

func NewPAMAuthenticator(service string) Authenticator {
	if service == "" {
		service = "login"
	}
	return &pamAuthenticator{Service: service}
}

func getTTY() string {
	if tty, err := os.Readlink("/proc/self/fd/0"); err == nil && strings.HasPrefix(tty, "/dev/") {
		return strings.TrimPrefix(tty, "/dev/")
	}
	return ""
}

func (a *pamAuthenticator) Authenticate(username, password string) error {
	var pamMsgs []string

	tx, err := pam.StartFunc(a.Service, username, func(style pam.Style, msg string) (string, error) {
		switch style {
		case pam.PromptEchoOff:
			return password, nil
		case pam.PromptEchoOn:
			return username, nil
		case pam.ErrorMsg, pam.TextInfo:
			if msg != "" {
				pamMsgs = append(pamMsgs, strings.TrimSpace(msg))
			}
			return "", nil
		default:
			return "", fmt.Errorf("unsupported PAM style: %v", style)
		}
	})
	if err != nil {
		return err
	}
	defer tx.End()

	if tty := getTTY(); tty != "" {
		_ = tx.SetItem(pam.Tty, tty)
	}

	if err := tx.Authenticate(0); err != nil {
		if len(pamMsgs) > 0 {
			return fmt.Errorf("%w: %s", err, strings.Join(pamMsgs, "; "))
		}
		return err
	}
	if err := tx.AcctMgmt(0); err != nil {
		if len(pamMsgs) > 0 {
			return fmt.Errorf("account management failed: %w: %s", err, strings.Join(pamMsgs, "; "))
		}
		return err
	}
	return nil
}
