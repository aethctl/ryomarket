package provider

import (
	"os"
	"testing"

	"github.com/Ryoku-dev/ryomarket/internal/model"
)

func TestPacmanPlanDoesNotUseShell(t *testing.T) {
	app := model.Application{ID: "obs-studio", Name: "OBS Studio"}
	v := model.Variant{Kind: model.ProviderPacman, Package: "obs-studio"}
	plan, err := Pacman().Plan(app, model.DistroArch, v, Install)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Executable != "pacman" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if !plan.Steps[0].Elevated {
		t.Fatal("pacman install should be elevated")
	}
}

func TestFlatpakPlanIsUserScoped(t *testing.T) {
	app := model.Application{ID: "gimp", Name: "GIMP"}
	v := model.Variant{Kind: model.ProviderFlatpak, Package: "org.gimp.GIMP"}
	plan, err := Flatpak().Plan(app, model.DistroFedora, v, Install)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, step := range plan.Steps {
		if step.Executable != "flatpak" || len(step.Args) == 0 || step.Args[0] != "install" {
			continue
		}
		found = true
		if step.Elevated {
			t.Fatal("flatpak user install should not be elevated")
		}
		if !contains(step.Args, "--user") {
			t.Fatalf("flatpak install must be user scoped: %#v", step.Args)
		}
	}
	if !found {
		t.Fatalf("flatpak install step missing: %#v", plan.Steps)
	}
}

func TestFlatpakBootstrapUsesNativeManager(t *testing.T) {
	cases := []struct {
		distro model.Distro
		binary string
	}{
		{model.DistroFedora, "dnf"},
		{model.DistroArch, "pacman"},
		{model.DistroCachyOS, "pacman"},
		{model.DistroVoid, "xbps-install"},
	}
	for _, tc := range cases {
		step, err := flatpakBootstrap(tc.distro)
		if err != nil {
			t.Fatalf("%s: %v", tc.distro, err)
		}
		if step.Executable != tc.binary || !step.Elevated {
			t.Fatalf("%s: unexpected bootstrap step %#v", tc.distro, step)
		}
	}
}

func TestUpstreamArchivePlanUsesManagedHelper(t *testing.T) {
	backend := Upstream()
	app := model.Application{ID: "dev.zed.Zed", Name: "Zed"}
	variant := model.Variant{
		Kind: model.ProviderUpstream, Package: "dev.zed.Zed",
		ReleaseRepo: "zed-industries/zed", AssetPattern: `^zed-linux-x86_64\.tar\.gz$`, Entrypoint: "zed.app/bin/zed",
	}
	plan, err := backend.Plan(app, model.DistroFedora, variant, Install)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Executable != "ryomarket-portable" || plan.Steps[0].Elevated {
		t.Fatalf("unexpected portable plan: %#v", plan.Steps)
	}
}

func TestNixProfileContainsExactElement(t *testing.T) {
	data := []byte(`{"elements":{"obs-studio":{"active":true,"attrPath":"legacyPackages.x86_64-linux.obs-studio"}},"version":3}`)
	if !nixProfileContains(data, "obs-studio") {
		t.Fatal("expected exact active profile element to match")
	}
}

func TestNixProfileContainsRejectsSubstring(t *testing.T) {
	data := []byte(`{"elements":{"obs-studio-plugin":{"active":true,"attrPath":"legacyPackages.x86_64-linux.obs-studio-plugin"}},"version":3}`)
	if nixProfileContains(data, "obs-studio") {
		t.Fatal("substring-only match must not count as installed")
	}
}

func TestNixProfileContainsInactiveElement(t *testing.T) {
	data := []byte(`{"elements":{"obs-studio":{"active":false,"attrPath":"legacyPackages.x86_64-linux.obs-studio"}},"version":3}`)
	if nixProfileContains(data, "obs-studio") {
		t.Fatal("inactive profile element must not count as installed")
	}
}

func TestPortableVariantRejectsWrongArchitecture(t *testing.T) {
	b := portableBackend{kind: model.ProviderAppImage}
	v := model.Variant{Architectures: []string{"definitely-not-this-arch"}}
	if b.AvailableVariant(v) {
		t.Fatal("expected mismatched architecture to be unavailable")
	}
}

func TestNixProfileContainsRejectsDifferentElementName(t *testing.T) {
	data := []byte(`{"elements":{"custom-name":{"active":true,"attrPath":"legacyPackages.x86_64-linux.obs-studio"}},"version":3}`)
	if nixProfileContains(data, "obs-studio") {
		t.Fatal("profile detection must use the removable element name")
	}
}

func TestPacmanUpdateWarnsAboutFullUpgrade(t *testing.T) {
	app := model.Application{ID: "obs-studio", Name: "OBS Studio"}
	v := model.Variant{Kind: model.ProviderPacman, Package: "obs-studio"}
	plan, err := Pacman().Plan(app, model.DistroArch, v, Update)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("expected Arch update warning")
	}
}

func TestXBPSUpdateTargetsOnlySelectedPackage(t *testing.T) {
	app := model.Application{ID: "vlc", Name: "VLC"}
	v := model.Variant{Kind: model.ProviderXBPS, Package: "vlc"}
	plan, err := XBPS().Plan(app, model.DistroVoid, v, Update)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("unexpected steps: %#v", plan.Steps)
	}
	args := plan.Steps[0].Args
	if len(args) != 2 || args[0] != "-Sy" || args[1] != "vlc" {
		t.Fatalf("expected targeted XBPS update, got %#v", args)
	}
}

func TestTrustedHelperRejectsWritableBinary(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ryomarket-nix"
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := trustedHelperPath(path); err == nil {
		t.Fatal("world-writable elevated helper must be rejected")
	}
}

func TestNixProfileSelectorUsesProfileName(t *testing.T) {
	v := model.Variant{Package: "kdePackages.kdenlive", ProfileName: "kdenlive"}
	if got := nixProfileSelector(v); got != "kdenlive" {
		t.Fatalf("expected kdenlive, got %q", got)
	}
}
