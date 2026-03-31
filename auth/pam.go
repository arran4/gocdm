//go:build !nopam

package auth

import (
	"fmt"

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

func (a *pamAuthenticator) Authenticate(username, password string) error {
	tx, err := pam.StartFunc(a.Service, username, func(style pam.Style, msg string) (string, error) {
		switch style {
		case pam.PromptEchoOff:
			return password, nil
		case pam.PromptEchoOn:
			return username, nil
		case pam.ErrorMsg, pam.TextInfo:
			return "", nil
		default:
			return "", fmt.Errorf("unsupported PAM style: %v", style)
		}
	})
	if err != nil {
		return err
	}

	if err := tx.Authenticate(0); err != nil {
		return err
	}
	return tx.AcctMgmt(0)
}
