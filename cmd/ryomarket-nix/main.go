package main

import (
	"fmt"
	"os"

	"github.com/Ryoku-dev/ryomarket/internal/nixconfig"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: ryomarket-nix <supported|setup|has|add|remove|rebuild> [package]")
	}
	var err error
	switch os.Args[1] {
	case "supported":
		if !nixconfig.Supported() {
			os.Exit(1)
		}
	case "setup":
		err = nixconfig.Setup()
	case "has":
		needPackage()
		if !nixconfig.Has(os.Args[2]) {
			os.Exit(1)
		}
	case "add":
		needPackage()
		err = nixconfig.Apply(os.Args[2], true)
	case "remove":
		needPackage()
		err = nixconfig.Apply(os.Args[2], false)
	case "rebuild":
		err = nixconfig.Rebuild()
	default:
		fail("unknown command: " + os.Args[1])
	}
	if err != nil {
		fail(err.Error())
	}
}

func needPackage() {
	if len(os.Args) != 3 {
		fail("package argument required")
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
