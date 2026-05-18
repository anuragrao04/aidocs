package main

import (
	"fmt"
	"os"

	"github.com/anuragrao/aidocs/cli/internal"
)

func main() {
	if err := internal.NewRoot(os.Stdout).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(internal.ExitCode(err))
	}
}
