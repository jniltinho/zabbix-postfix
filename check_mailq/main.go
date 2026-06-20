package main

import (
	"fmt"
	"os"

	"check_mailq/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		if ec, ok := err.(interface{ ExitCode() int }); ok {
			os.Exit(ec.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(stateUnknown)
	}
}

const stateUnknown = 3
