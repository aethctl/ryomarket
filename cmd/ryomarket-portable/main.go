package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Ryoku-dev/ryomarket/internal/portable"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: ryomarket-portable <command> ...")
	}
	m := portable.New()
	ctx := context.Background()

	switch os.Args[1] {
	case "installed":
		if len(os.Args) != 4 {
			fail("usage: ryomarket-portable installed <appimage|archive> <id>")
		}
		if !m.Installed(os.Args[3], os.Args[2]) {
			os.Exit(1)
		}
	case "remove":
		if len(os.Args) != 3 {
			fail("usage: ryomarket-portable remove <id>")
		}
		if err := m.Remove(os.Args[2]); err != nil {
			fail(err.Error())
		}
	case "install-appimage", "update-appimage":
		if len(os.Args) != 7 {
			fail("usage: ryomarket-portable install-appimage <id> <name> <url> <repo> <asset-pattern>")
		}
		source := portable.Source{URL: os.Args[4], ReleaseRepo: os.Args[5], AssetPattern: os.Args[6]}
		if err := m.InstallAppImage(ctx, os.Args[2], os.Args[3], source); err != nil {
			fail(err.Error())
		}
	case "install-archive", "update-archive":
		if len(os.Args) != 8 {
			fail("usage: ryomarket-portable install-archive <id> <name> <entrypoint> <url> <repo> <asset-pattern>")
		}
		source := portable.Source{URL: os.Args[5], ReleaseRepo: os.Args[6], AssetPattern: os.Args[7]}
		if err := m.InstallArchive(ctx, os.Args[2], os.Args[3], os.Args[4], source); err != nil {
			fail(err.Error())
		}
	case "update-available":
		if len(os.Args) != 6 {
			fail("usage: ryomarket-portable update-available <id> <url> <repo> <asset-pattern>")
		}
		available, err := m.UpdateAvailable(ctx, os.Args[2], portable.Source{URL: os.Args[3], ReleaseRepo: os.Args[4], AssetPattern: os.Args[5]})
		if err != nil {
			fail(err.Error())
		}
		if available {
			os.Exit(10)
		}
	case "supported":
		if !portable.ArchMatches() {
			os.Exit(1)
		}
	default:
		fail("unknown command " + os.Args[1])
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
