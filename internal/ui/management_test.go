package ui

import (
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopLaunchUsesAnExistingApplicationFile(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	data := filepath.Join(dir, "data")
	os.MkdirAll(bin, 0700)
	os.MkdirAll(filepath.Join(data, "applications"), 0700)
	os.WriteFile(filepath.Join(bin, "gio"), []byte("#!/bin/sh\nexit 0\n"), 0700)
	t.Setenv("PATH", bin)
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_DATA_DIRS", data)
	desktop := filepath.Join(data, "applications", "example.desktop")
	os.WriteFile(desktop, []byte("[Desktop Entry]\nType=Application\nExec=/bin/true\n"), 0600)
	args, ok := launchCommand(model.Application{ID: "example"}, model.Variant{Kind: model.ProviderNixProfile, Package: "example"})
	if !ok || len(args) != 3 || args[1] != "launch" || args[2] != desktop {
		t.Fatalf("incorrect launch command: %v", args)
	}
	if safeDesktopID("../../payload") {
		t.Fatal("unsafe desktop identity accepted")
	}
	os.WriteFile(desktop, []byte("[Desktop Entry]\nType=Link\nURL=https://example.org\n"), 0600)
	if desktopApplication(desktop) {
		t.Fatal("a link was treated as an application")
	}
}
