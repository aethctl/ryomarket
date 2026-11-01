package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/Ryoku-dev/ryomarket/internal/model"
)

type commandBackend struct {
	kind            model.ProviderKind
	binary          string
	available       func() bool
	install         func(model.Variant) Step
	remove          func(model.Variant) Step
	update          func(model.Variant) Step
	isInstalled     func(context.Context, model.Variant) bool
	updateAvailable func(context.Context, model.Variant) bool
	warnings        func(Operation, model.Variant) []string
}

func (b commandBackend) Kind() model.ProviderKind { return b.kind }
func (b commandBackend) Available() bool {
	if b.available != nil {
		return b.available()
	}
	_, err := exec.LookPath(b.binary)
	return err == nil
}
func (b commandBackend) UpdateAvailable(ctx context.Context, v model.Variant) bool {
	if b.updateAvailable == nil {
		return false
	}
	return b.updateAvailable(ctx, v)
}
func (b commandBackend) Installed(ctx context.Context, v model.Variant) bool {
	if b.isInstalled == nil {
		return false
	}
	return b.isInstalled(ctx, v)
}
func (b commandBackend) Plan(app model.Application, distro model.Distro, v model.Variant, op Operation) (Plan, error) {
	var step Step
	switch op {
	case Install:
		if b.install == nil {
			return Plan{}, fmt.Errorf("%s does not support install", b.kind)
		}
		step = b.install(v)
	case Remove:
		if b.remove == nil {
			return Plan{}, fmt.Errorf("%s does not support removal", b.kind)
		}
		step = b.remove(v)
	case Update:
		if b.update == nil {
			return Plan{}, fmt.Errorf("%s does not support individual updates", b.kind)
		}
		step = b.update(v)
	default:
		return Plan{}, fmt.Errorf("unknown operation %q", op)
	}
	warnings := []string(nil)
	if b.warnings != nil {
		warnings = b.warnings(op, v)
	}
	return Plan{AppID: app.ID, AppName: app.Name, Distro: distro, Variant: v, Operation: op, Steps: []Step{step}, Warnings: warnings}, nil
}

func DNF() Backend {
	return commandBackend{
		kind: model.ProviderDNF, binary: "dnf",
		install: func(v model.Variant) Step {
			return Step{"Install " + v.Package, "dnf", []string{"install", "-y", v.Package}, true}
		},
		remove: func(v model.Variant) Step {
			return Step{"Remove " + v.Package, "dnf", []string{"remove", "-y", v.Package}, true}
		},
		update: func(v model.Variant) Step {
			return Step{"Update " + v.Package, "dnf", []string{"upgrade", "-y", v.Package}, true}
		},
		isInstalled: commandStatus("rpm", "-q"),
		updateAvailable: func(ctx context.Context, v model.Variant) bool {
			err := exec.CommandContext(ctx, "dnf", "check-upgrade", "--quiet", v.Package).Run()
			if exit, ok := err.(*exec.ExitError); ok {
				return exit.ExitCode() == 100
			}
			return false
		},
	}
}

func Pacman() Backend {
	return commandBackend{
		kind: model.ProviderPacman, binary: "pacman",
		install: func(v model.Variant) Step {
			return Step{"Install " + v.Package, "pacman", []string{"-S", "--needed", "--noconfirm", v.Package}, true}
		},
		remove: func(v model.Variant) Step {
			return Step{"Remove " + v.Package, "pacman", []string{"-Rns", "--noconfirm", v.Package}, true}
		},
		update: func(v model.Variant) Step {
			return Step{"Upgrade Arch system and " + v.Package, "pacman", []string{"-Syu", "--noconfirm", v.Package}, true}
		},
		warnings: func(op Operation, _ model.Variant) []string {
			if op == Update {
				return []string{"Arch-based systems do not support partial upgrades. This transaction can update other system packages too."}
			}
			return nil
		},
		isInstalled: commandStatus("pacman", "-Q"),
		updateAvailable: func(ctx context.Context, v model.Variant) bool {
			out, err := exec.CommandContext(ctx, "pacman", "-Qu", v.Package).Output()
			return err == nil && strings.Contains(string(out), v.Package+" ")
		},
	}
}

func XBPS() Backend {
	return commandBackend{
		kind: model.ProviderXBPS, binary: "xbps-install",
		install: func(v model.Variant) Step {
			return Step{"Install " + v.Package, "xbps-install", []string{"-Sy", v.Package}, true}
		},
		remove: func(v model.Variant) Step {
			return Step{"Remove " + v.Package, "xbps-remove", []string{"-Ry", v.Package}, true}
		},
		update: func(v model.Variant) Step {
			return Step{"Update " + v.Package, "xbps-install", []string{"-Sy", v.Package}, true}
		},
		isInstalled: func(ctx context.Context, v model.Variant) bool {
			return exec.CommandContext(ctx, "xbps-query", "-p", "pkgver", v.Package).Run() == nil
		},
		updateAvailable: func(ctx context.Context, v model.Variant) bool {
			out, err := exec.CommandContext(ctx, "xbps-install", "-Mn", v.Package).Output()
			return err == nil && strings.Contains(string(out), v.Package)
		},
	}
}

type flatpakBackend struct{}

func (flatpakBackend) Kind() model.ProviderKind { return model.ProviderFlatpak }
func (flatpakBackend) Available() bool          { return runtime.GOOS == "linux" }
func (flatpakBackend) AvailableFor(distro model.Distro) bool {
	if executableAvailable("flatpak") {
		return true
	}
	switch distro {
	case model.DistroFedora:
		return executableAvailable("dnf")
	case model.DistroArch, model.DistroCachyOS:
		return executableAvailable("pacman")
	case model.DistroVoid:
		return executableAvailable("xbps-install")
	default:
		return false
	}
}
func (flatpakBackend) Installed(ctx context.Context, v model.Variant) bool {
	if exec.CommandContext(ctx, "flatpak", "info", "--user", v.Package).Run() == nil {
		return true
	}
	return exec.CommandContext(ctx, "flatpak", "info", "--system", v.Package).Run() == nil
}
func (flatpakBackend) UpdateAvailable(ctx context.Context, v model.Variant) bool {
	scope := "--system"
	if flatpakUserInstalled(v.Package) {
		scope = "--user"
	}
	out, err := exec.CommandContext(ctx, "flatpak", "remote-ls", scope, "--updates", "--columns=application").Output()
	return err == nil && strings.Contains("\n"+string(out)+"\n", "\n"+v.Package+"\n")
}
func (flatpakBackend) Plan(app model.Application, distro model.Distro, v model.Variant, op Operation) (Plan, error) {
	plan := Plan{AppID: app.ID, AppName: app.Name, Distro: distro, Variant: v, Operation: op}
	switch op {
	case Install:
		if !executableAvailable("flatpak") {
			step, err := flatpakBootstrap(distro)
			if err != nil {
				return Plan{}, err
			}
			plan.Steps = append(plan.Steps, step)
		}
		if !flatpakUserRemote("flathub") {
			plan.Steps = append(plan.Steps, Step{
				Title: "Enable Flathub for this user", Executable: "flatpak",
				Args: []string{"remote-add", "--user", "--if-not-exists", "flathub", "https://dl.flathub.org/repo/flathub.flatpakrepo"},
			})
		}
		plan.Steps = append(plan.Steps, Step{"Install " + v.Package, "flatpak", []string{"install", "--user", "-y", "--noninteractive", "flathub", v.Package}, false})
	case Remove:
		scope := "--system"
		if flatpakUserInstalled(v.Package) {
			scope = "--user"
		}
		plan.Steps = []Step{{"Remove " + v.Package, "flatpak", []string{"uninstall", scope, "-y", "--noninteractive", v.Package}, false}}
	case Update:
		scope := "--system"
		if flatpakUserInstalled(v.Package) {
			scope = "--user"
		}
		plan.Steps = []Step{{"Update " + v.Package, "flatpak", []string{"update", scope, "-y", "--noninteractive", v.Package}, false}}
	default:
		return Plan{}, fmt.Errorf("unknown operation %q", op)
	}
	return plan, nil
}

func Flatpak() Backend { return flatpakBackend{} }

func flatpakBootstrap(distro model.Distro) (Step, error) {
	switch distro {
	case model.DistroFedora:
		return Step{"Install Flatpak", "dnf", []string{"install", "-y", "flatpak"}, true}, nil
	case model.DistroArch, model.DistroCachyOS:
		return Step{"Install Flatpak", "pacman", []string{"-S", "--needed", "--noconfirm", "flatpak"}, true}, nil
	case model.DistroVoid:
		return Step{"Install Flatpak", "xbps-install", []string{"-Sy", "flatpak"}, true}, nil
	default:
		return Step{}, fmt.Errorf("Flatpak is not installed and Ryomarket cannot bootstrap it on %s", distro.DisplayName())
	}
}

func executableAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func flatpakUserRemote(name string) bool {
	out, err := exec.Command("flatpak", "remotes", "--user", "--columns=name").Output()
	if err != nil {
		return false
	}
	return strings.Contains("\n"+string(out)+"\n", "\n"+name+"\n")
}

func flatpakUserInstalled(packageID string) bool {
	return exec.Command("flatpak", "info", "--user", packageID).Run() == nil
}

func nixArgs(args ...string) []string {
	return append([]string{"--extra-experimental-features", "nix-command flakes"}, args...)
}

func NixProfile() Backend {
	return commandBackend{
		kind: model.ProviderNixProfile, binary: "nix",
		install: func(v model.Variant) Step {
			return Step{"Add " + v.Package + " to Nix profile", "nix", nixArgs("profile", "add", nixInstallable(v)), false}
		},
		remove: func(v model.Variant) Step {
			selector := nixProfileSelector(v)
			return Step{"Remove " + selector + " from Nix profile", "nix", nixArgs("profile", "remove", selector), false}
		},
		update: func(v model.Variant) Step {
			selector := nixProfileSelector(v)
			return Step{"Update " + selector, "nix", nixArgs("profile", "upgrade", selector), false}
		},
		isInstalled: func(ctx context.Context, v model.Variant) bool {
			out, err := exec.CommandContext(ctx, "nix", nixArgs("profile", "list", "--json")...).Output()
			return err == nil && nixProfileContains(out, nixProfileSelector(v))
		},
	}
}

func nixInstallable(v model.Variant) string {
	if v.FlakeRef != "" {
		return v.FlakeRef + "#" + v.Package
	}
	return "nixpkgs#" + v.Package
}

func nixProfileSelector(v model.Variant) string {
	if value := strings.TrimSpace(v.ProfileName); value != "" {
		return value
	}
	value := strings.TrimSpace(v.Package)
	if index := strings.LastIndexByte(value, '.'); index >= 0 && index+1 < len(value) {
		return value[index+1:]
	}
	return value
}

type nixProfileElement struct {
	Active bool `json:"active"`
}

func nixProfileContains(data []byte, packageName string) bool {
	var document struct {
		Elements map[string]nixProfileElement `json:"elements"`
	}
	if err := json.Unmarshal(data, &document); err == nil && document.Elements != nil {
		return nixProfileElementsContain(document.Elements, packageName)
	}

	var legacy map[string]nixProfileElement
	if json.Unmarshal(data, &legacy) != nil {
		return false
	}
	return nixProfileElementsContain(legacy, packageName)
}

func nixProfileElementsContain(elements map[string]nixProfileElement, packageName string) bool {
	element, ok := elements[packageName]
	return ok && element.Active
}

func NixConfig() Backend {
	return commandBackend{
		kind: model.ProviderNixConfig, binary: "ryomarket-nix",
		available: func() bool {
			helper, err := trustedHelper("ryomarket-nix")
			return err == nil && exec.Command(helper, "supported").Run() == nil
		},
		install: func(v model.Variant) Step {
			return Step{"Add " + v.Package + " to NixOS configuration", "ryomarket-nix", []string{"add", v.Package}, true}
		},
		remove: func(v model.Variant) Step {
			return Step{"Remove " + v.Package + " from NixOS configuration", "ryomarket-nix", []string{"remove", v.Package}, true}
		},
		update: func(v model.Variant) Step { return Step{"Rebuild NixOS", "ryomarket-nix", []string{"rebuild"}, true} },
		isInstalled: func(ctx context.Context, v model.Variant) bool {
			helper, err := trustedHelper("ryomarket-nix")
			return err == nil && exec.CommandContext(ctx, helper, "has", v.Package).Run() == nil
		},
	}
}

type portableBackend struct {
	kind model.ProviderKind
}

func (b portableBackend) Kind() model.ProviderKind { return b.kind }
func (b portableBackend) AvailableVariant(v model.Variant) bool {
	if len(v.Architectures) > 0 && !contains(v.Architectures, runtime.GOARCH) {
		return false
	}
	if len(v.Libcs) > 0 {
		libc := currentLibc()
		if libc == "" || !contains(v.Libcs, libc) {
			return false
		}
	}
	return true
}
func (b portableBackend) Available() bool {
	helper, err := localHelper("ryomarket-portable")
	return err == nil && exec.Command(helper, "supported").Run() == nil
}
func (b portableBackend) Installed(ctx context.Context, v model.Variant) bool {
	if !b.AvailableVariant(v) {
		return false
	}
	kind := "appimage"
	if b.kind == model.ProviderUpstream {
		kind = "archive"
	}
	helper, err := localHelper("ryomarket-portable")
	return err == nil && exec.CommandContext(ctx, helper, "installed", kind, v.Package).Run() == nil
}
func (b portableBackend) UpdateAvailable(ctx context.Context, v model.Variant) bool {
	if !b.AvailableVariant(v) || v.ReleaseRepo == "" {
		return false
	}
	helper, helperErr := localHelper("ryomarket-portable")
	if helperErr != nil {
		return false
	}
	err := exec.CommandContext(ctx, helper, "update-available", v.Package, v.SourceURL, v.ReleaseRepo, v.AssetPattern).Run()
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode() == 10
	}
	return false
}
func (b portableBackend) Plan(app model.Application, distro model.Distro, v model.Variant, op Operation) (Plan, error) {
	if !b.AvailableVariant(v) {
		return Plan{}, fmt.Errorf("%s is not available on this host", b.kind.DisplayName())
	}
	if v.Package == "" {
		return Plan{}, fmt.Errorf("%s recipe is missing a managed package id", b.kind.DisplayName())
	}
	if v.SourceURL == "" && (v.ReleaseRepo == "" || v.AssetPattern == "") {
		return Plan{}, fmt.Errorf("%s recipe has no resolvable source", b.kind.DisplayName())
	}

	var step Step
	switch op {
	case Remove:
		step = Step{"Remove " + app.Name, "ryomarket-portable", []string{"remove", v.Package}, false}
	case Install, Update:
		verb := "install"
		if op == Update {
			verb = "update"
		}
		if b.kind == model.ProviderAppImage {
			step = Step{operationLabel(verb) + " " + app.Name, "ryomarket-portable", []string{verb + "-appimage", v.Package, app.Name, v.SourceURL, v.ReleaseRepo, v.AssetPattern}, false}
		} else {
			if v.Entrypoint == "" {
				return Plan{}, fmt.Errorf("upstream archive recipe is missing an entrypoint")
			}
			step = Step{operationLabel(verb) + " " + app.Name, "ryomarket-portable", []string{verb + "-archive", v.Package, app.Name, v.Entrypoint, v.SourceURL, v.ReleaseRepo, v.AssetPattern}, false}
		}
	default:
		return Plan{}, fmt.Errorf("unknown operation %q", op)
	}

	warnings := []string(nil)
	if b.kind == model.ProviderUpstream {
		warnings = append(warnings, "This is a curated upstream binary archive, not a distro-managed package.")
	}
	return Plan{AppID: app.ID, AppName: app.Name, Distro: distro, Variant: v, Operation: op, Steps: []Step{step}, Warnings: warnings}, nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

var (
	libcOnce sync.Once
	libcName string
)

func currentLibc() string {
	libcOnce.Do(func() {
		if runtime.GOOS != "linux" {
			return
		}
		if matches, _ := filepath.Glob("/lib/ld-musl-*.so.1"); len(matches) > 0 {
			libcName = "musl"
			return
		}
		out, err := exec.Command("ldd", "--version").CombinedOutput()
		text := strings.ToLower(string(out))
		if strings.Contains(text, "musl") {
			libcName = "musl"
			return
		}
		if err == nil || strings.Contains(text, "glibc") || strings.Contains(text, "gnu libc") {
			libcName = "glibc"
		}
	})
	return libcName
}

func AppImage() Backend { return portableBackend{kind: model.ProviderAppImage} }
func Upstream() Backend { return portableBackend{kind: model.ProviderUpstream} }

func Registry() map[model.ProviderKind]Backend {
	backends := []Backend{DNF(), Pacman(), XBPS(), NixFlake(), NixProfile(), NixConfig(), Flatpak(), AppImage(), Upstream()}
	out := make(map[model.ProviderKind]Backend, len(backends))
	for _, backend := range backends {
		out[backend.Kind()] = backend
	}
	return out
}

func commandStatus(binary string, prefix ...string) func(context.Context, model.Variant) bool {
	return func(ctx context.Context, v model.Variant) bool {
		args := append(append([]string(nil), prefix...), v.Package)
		return exec.CommandContext(ctx, binary, args...).Run() == nil
	}
}

func operationLabel(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
