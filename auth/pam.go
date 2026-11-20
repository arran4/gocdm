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
	for _, fd := range []uintptr{os.Stdin.Fd(), os.Stdout.Fd()} {
		if tty, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd)); err == nil && strings.HasPrefix(tty, "/dev/") {
			return strings.TrimPrefix(tty, "/dev/")
		}
	}
	return ""
}

type pamTxAdapter struct {
	tx *pam.Transaction
}

func (a *pamTxAdapter) SetItemTTY(tty string) error {
	return a.tx.SetItem(pam.Tty, tty)
}

func (a *pamTxAdapter) GetItemUser() (string, error) {
	return a.tx.GetItem(pam.User)
}

func (a *pamTxAdapter) Authenticate() error {
	return a.tx.Authenticate(0)
}

func (a *pamTxAdapter) AcctMgmt() error {
	return a.tx.AcctMgmt(0)
}

func (a *pamTxAdapter) PutEnv(nameval string) error {
	return a.tx.PutEnv(nameval)
}

func (a *pamTxAdapter) SetCred(flag pamCredFlag) error {
	switch flag {
	case credEstablish:
		return a.tx.SetCred(pam.EstablishCred)
	case credDelete:
		return a.tx.SetCred(pam.DeleteCred)
	default:
		return fmt.Errorf("unsupported PAM credential flag: %v", flag)
	}
}

func (a *pamTxAdapter) OpenSession() error {
	return a.tx.OpenSession(0)
}

func (a *pamTxAdapter) GetEnvList() (map[string]string, error) {
	return a.tx.GetEnvList()
}

func (a *pamTxAdapter) CloseSession() error {
	return a.tx.CloseSession(0)
}

func (a *pamTxAdapter) End() error {
	return a.tx.End()
}

func mapPamStyle(style pam.Style) (pamConvStyle, bool) {
	switch style {
	case pam.PromptEchoOff:
		return convPromptEchoOff, true
	case pam.PromptEchoOn:
		return convPromptEchoOn, true
	case pam.ErrorMsg:
		return convErrorMsg, true
	case pam.TextInfo:
		return convTextInfo, true
	default:
		return 0, false
	}
}

func (a *pamAuthenticator) Authenticate(username, password string) (LoginSession, error) {
	return runPAMAuthenticate(username, password, getTTY(), func(conv func(pamConvStyle, string) (string, error)) (pamHandle, error) {
		tx, err := pam.StartFunc(a.Service, username, func(style pam.Style, msg string) (string, error) {
			mapped, ok := mapPamStyle(style)
			if !ok {
				return "", fmt.Errorf("unsupported PAM style: %v", style)
			}
			return conv(mapped, msg)
		})
		if err != nil {
			return nil, err
		}
		return &pamTxAdapter{tx: tx}, nil
	})
}
