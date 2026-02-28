//go:build !pam

package auth

import "fmt"

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
	return fmt.Errorf("pam authentication is not available in this build (rebuild with -tags pam)")
}
