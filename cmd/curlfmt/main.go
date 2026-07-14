// Command curlfmt formats, lints, and canonicalizes curl commands found on
// stdin, in Markdown documents, and in shell scripts.
package main

import (
	"os"

	"github.com/JaydenCJ/curlfmt/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
