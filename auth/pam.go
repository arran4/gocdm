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
	if tty, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", os.Stdin.Fd())); err == nil && strings.HasPrefix(tty, "/dev/") {
		return strings.TrimPrefix(tty, "/dev/")
	}
	return ""
}

type pamLoginSession struct {
	tx *pam.Transaction
}

func (s *pamLoginSession) OpenSession() error {
	return s.tx.OpenSession(0)
}

func (s *pamLoginSession) Env() []string {
	envMap, err := s.tx.GetEnvList()
	if err != nil {
		return nil
	}
	var env []string
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

func (s *pamLoginSession) CloseSession() error {
	err := s.tx.CloseSession(0)
	if endErr := s.tx.End(); err == nil {
		err = endErr
	}
	return err
}

func (a *pamAuthenticator) Authenticate(username, password string) (LoginSession, error) {
	var pamMsgs []string

	tx, err := pam.StartFunc(a.Service, username, func(style pam.Style, msg string) (string, error) {
		switch style {
		case pam.PromptEchoOff:
			return password, nil
		case pam.PromptEchoOn:
			return username, nil
		case pam.ErrorMsg, pam.TextInfo:
			if s := strings.TrimSpace(msg); s != "" {
				pamMsgs = append(pamMsgs, s)
			}
			return "", nil
		default:
			return "", fmt.Errorf("unsupported PAM style: %v", style)
		}
	})
	if err != nil {
		return nil, err
	}
	// tx will be closed by LoginSession.CloseSession() on success

	if tty := getTTY(); tty != "" {
		_ = tx.SetItem(pam.Tty, tty)
	}

	if err := tx.Authenticate(0); err != nil {
		endErr := tx.End()
		if endErr != nil {
			err = fmt.Errorf("PAM auth error: %w, subsequent end error: %v", err, endErr)
		}
		if len(pamMsgs) > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.Join(pamMsgs, "; "))
		}
		return nil, err
	}
	pamMsgs = nil
	if err := tx.AcctMgmt(0); err != nil {
		endErr := tx.End()
		if endErr != nil {
			err = fmt.Errorf("PAM acct mgmt error: %w, subsequent end error: %v", err, endErr)
		}
		if len(pamMsgs) > 0 {
			return nil, fmt.Errorf("account management failed: %w: %s", err, strings.Join(pamMsgs, "; "))
		}
		return nil, err
	}
	return &pamLoginSession{tx: tx}, nil
}
