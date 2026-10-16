package main

import (
	"fmt"
	"os"

	"github.com/Ryoku-dev/ryomarket/internal/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
