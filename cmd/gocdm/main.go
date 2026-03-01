package main

import (
	"os"

	"github.com/arran4/gocdm/cli"
)

func main() {
	cli.Run(os.Args[1:], os.Exit)
}
