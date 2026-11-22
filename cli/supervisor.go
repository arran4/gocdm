package cli

import (
	"io"

	"github.com/arran4/gocdm/config"
)

// SupervisedSessionSpec describes a child session process managed under a PAM login supervisor.
type SupervisedSessionSpec struct {
	Binary  string
	Args    []string
	Env     []string
	Dir     string
	Account *config.AccountInfo
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}
