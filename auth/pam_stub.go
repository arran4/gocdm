//go:build nopam

package auth

import "fmt"

type stubAuthenticator struct{}

func NewPAMAuthenticator(service string) Authenticator {
	return stubAuthenticator{}
}

func (stubAuthenticator) Authenticate(username, password string) error {
	return fmt.Errorf("pam authentication is not available in this build (rebuild without -tags nopam)")
}
