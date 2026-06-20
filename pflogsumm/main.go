package main

import (
	"fmt"
	"os"

	"pflogsumm/cmd"
)

// exitCoder is implemented by nagiosExit to signal a specific exit code.
type exitCoder interface {
	ExitCode() int
}

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		if ec, ok := err.(exitCoder); ok {
			os.Exit(ec.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
