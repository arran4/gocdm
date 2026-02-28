//go:build pam

package auth

import (
	"fmt"

	"github.com/msteinert/pam/v2"
)

// PAMAuthenticator authenticates users against PAM.
type PAMAuthenticator struct {
	Service string
}

func NewPAMAuthenticator(service string) *PAMAuthenticator {
	if service == "" {
		service = "login"
	}
	return &PAMAuthenticator{Service: service}
}

func (a *PAMAuthenticator) Authenticate(username, password string) error {
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
