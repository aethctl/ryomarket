package provider

import (
	"context"
	"fmt"
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type nixFlakeBackend struct{}

func (nixFlakeBackend) Kind() model.ProviderKind { return model.ProviderNixFlake }
func (nixFlakeBackend) Available() bool {
	return runtime.GOOS == "linux" && runtime.GOARCH == "amd64" && executableAvailable("nix")
}
func (nixFlakeBackend) AvailableVariant(v model.Variant) bool {
	return validFlakeApp(v.Package) && v.FlakeRef != ""
}
func validFlakeApp(name string) bool {
	switch name {
	case "ryotunes", "ryomanager", "ryostore", "ryomotion", "rashin", "ryoport":
		return true
	}
	return false
}
func flakeAppRoot(name string) string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "ryomarket", "flake-apps", name)
}
func (nixFlakeBackend) Installed(_ context.Context, v model.Variant) bool {
	if !validFlakeApp(v.Package) || v.Entrypoint == "" {
		return false
	}
	root := flakeAppRoot(v.Package)
	owner, err := os.ReadFile(filepath.Join(root, "managed-by"))
	if err != nil || strings.TrimSpace(string(owner)) != "ryomarket:"+v.Package {
		return false
	}
	info, err := os.Stat(filepath.Join(root, "current", "profile", "bin", v.Entrypoint))
	return err == nil && !info.IsDir() && info.Mode()&0111 != 0
}
func (b nixFlakeBackend) UpdateAvailable(ctx context.Context, v model.Variant) bool {
	if !b.Installed(ctx, v) {
		return false
	}
	ref, err := os.ReadFile(filepath.Join(flakeAppRoot(v.Package), "current", "source"))
	return err == nil && strings.TrimSpace(string(ref)) != v.FlakeRef
}
func (b nixFlakeBackend) Plan(app model.Application, distro model.Distro, v model.Variant, operation Operation) (Plan, error) {
	if !b.AvailableVariant(v) {
		return Plan{}, fmt.Errorf("no bundled flake installer for %s", app.Name)
	}
	args := nixArgs("run", "--no-write-lock-file", v.FlakeRef+"#install-"+v.Package)
	switch operation {
	case Install, Update:
	case Remove:
		args = append(args, "--", "--remove")
	default:
		return Plan{}, fmt.Errorf("unsupported flake operation %s", operation)
	}
	return Plan{AppID: app.ID, AppName: app.Name, Distro: distro, Variant: v, Operation: operation, Steps: []Step{{Title: operationLabel(string(operation)) + " " + app.Name, Executable: "nix", Args: args}}}, nil
}
func NixFlake() Backend { return nixFlakeBackend{} }
