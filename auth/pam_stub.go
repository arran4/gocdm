//go:build nopam

package auth

import "fmt"

type stubAuthenticator struct{}

func NewPAMAuthenticator(service string) Authenticator {
	return stubAuthenticator{}
}

type stubLoginSession struct{}

func (s *stubLoginSession) OpenSession() error {
	return nil
}

func (s *stubLoginSession) Env() []string {
	return nil
}

func (s *stubLoginSession) CloseSession() error {
	return nil
}

func (stubAuthenticator) Authenticate(username, password string) (LoginSession, error) {
	return nil, fmt.Errorf("pam authentication is not available in this build (rebuild without -tags nopam)")
}
