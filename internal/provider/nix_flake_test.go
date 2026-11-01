package provider

import (
	"context"
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"os"
	"path/filepath"
	"testing"
)

func TestNixFlakePlanUsesBundledInstaller(t *testing.T) {
	v := model.Variant{Kind: model.ProviderNixFlake, Package: "ryotunes", FlakeRef: "path:/nix/store/source?narHash=sha256-test", Entrypoint: "ryotunes"}
	b := nixFlakeBackend{}
	for _, operation := range []Operation{Install, Update, Remove} {
		plan, err := b.Plan(model.Application{ID: "ryoku:ryotunes", Name: "Ryotunes"}, model.DistroNixOS, v, operation)
		if err != nil {
			t.Fatal(err)
		}
		step := plan.Steps[0]
		if step.Elevated || step.Executable != "nix" {
			t.Fatalf("unexpected step: %#v", step)
		}
		if !contains(step.Args, v.FlakeRef+"#install-ryotunes") {
			t.Fatalf("wrong installer: %v", step.Args)
		}
		if contains(step.Args, "--remove") != (operation == Remove) {
			t.Fatalf("wrong operation: %v", step.Args)
		}
	}
	v.Package = "../../foreign"
	if _, err := b.Plan(model.Application{}, model.DistroNixOS, v, Install); err == nil {
		t.Fatal("unmanaged package accepted")
	}
}

func TestNixFlakeDetectionRequiresOwnedExecutable(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	v := model.Variant{Package: "ryotunes", Entrypoint: "ryotunes", FlakeRef: "path:/source"}
	b := nixFlakeBackend{}
	root := flakeAppRoot(v.Package)
	if err := os.MkdirAll(filepath.Join(root, "current/profile/bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "current/profile/bin/ryotunes"), []byte("binary"), 0700); err != nil {
		t.Fatal(err)
	}
	if b.Installed(context.Background(), v) {
		t.Fatal("unmanaged executable counted as installed")
	}
	if err := os.WriteFile(filepath.Join(root, "managed-by"), []byte("ryomarket:ryotunes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !b.Installed(context.Background(), v) {
		t.Fatal("managed application not detected")
	}
}
