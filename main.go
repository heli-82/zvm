package main

import (
	"fmt"
	"os"

	"zvm/internal/executor"
)

func main() {
	e, err := executor.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "zvm:", err)
		os.Exit(1)
	}
	defer e.Deinit()
	e.ParseCommand()
}
