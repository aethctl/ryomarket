package catalog

import (
	"testing"

	"github.com/Ryoku-dev/ryomarket/internal/model"
)

func TestRepoPackagesFiltersLibrariesAndKeepsApps(t *testing.T) {
	apps := repoPackagesToApps([]repoPackage{
		{Name: "firefox", Package: "firefox", Summary: "Web browser for the desktop", Provider: model.ProviderNixProfile, NativeLabel: "Nixpkgs"},
		{Name: "libfoo", Package: "libfoo", Summary: "Runtime library for Foo", Provider: model.ProviderNixProfile, NativeLabel: "Nixpkgs"},
		{Name: "btop", Package: "btop", Summary: "Resource monitor and system tool", Provider: model.ProviderNixProfile, NativeLabel: "Nixpkgs"},
	}, model.DistroNixOS)
	if len(apps) != 2 {
		t.Fatalf("expected two app-like packages, got %#v", apps)
	}
}

func TestCategoriesFromRepoText(t *testing.T) {
	cats := categoriesFromRepoText("Matrix chat messaging client for the internet")
	want := map[string]bool{"Communication": true, "Social": true, "Internet": true}
	for _, cat := range cats {
		delete(want, cat)
	}
	if len(want) != 0 {
		t.Fatalf("missing categories: %#v from %#v", want, cats)
	}
}

func TestParseNixSearchNormalizesFlakeAttributes(t *testing.T) {
	data := []byte(`{
		"legacyPackages.x86_64-linux.firefox":{"pname":"firefox","version":"145.0","description":"Web browser for the desktop"},
		"packages.x86_64-linux.kitty":{"pname":"kitty","version":"0.44","description":"GPU based terminal emulator"}
	}`)
	packages, err := parseNixSearch(data)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, pkg := range packages {
		got[pkg.Package] = true
	}
	if !got["firefox"] || !got["kitty"] {
		t.Fatalf("unexpected normalized packages: %#v", packages)
	}
}

func TestNixPackagesAreAvailableAcrossDistros(t *testing.T) {
	apps := nixPackagesToApps([]repoPackage{{
		Name: "blender", Package: "blender", Summary: "3D graphics application", Version: "4.5",
		Provider: model.ProviderNixProfile, NativeLabel: "Nixpkgs",
	}})
	if len(apps) != 1 {
		t.Fatalf("expected one Nixpkgs app, got %d", len(apps))
	}
	for _, distro := range []model.Distro{model.DistroFedora, model.DistroArch, model.DistroCachyOS, model.DistroVoid} {
		variants := apps[0].Variants[distro]
		if len(variants) != 1 || variants[0].Kind != model.ProviderNixProfile || variants[0].Label != "Nixpkgs" {
			t.Fatalf("unexpected %s variants: %#v", distro, variants)
		}
	}
	nixos := apps[0].Variants[model.DistroNixOS]
	if len(nixos) != 2 || nixos[0].Kind != model.ProviderNixConfig || nixos[1].Kind != model.ProviderNixProfile {
		t.Fatalf("unexpected NixOS variants: %#v", nixos)
	}
}
