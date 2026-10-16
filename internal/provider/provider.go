package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Ryoku-dev/ryomarket/internal/model"
)

type Operation string

const (
	Install Operation = "install"
	Remove  Operation = "remove"
	Update  Operation = "update"
)

type Step struct {
	Title      string
	Executable string
	Args       []string
	Elevated   bool
}

type Plan struct {
	AppID     string
	AppName   string
	Distro    model.Distro
	Variant   model.Variant
	Operation Operation
	Steps     []Step
	Warnings  []string
}

func (p Plan) Summary() string {
	verb := operationLabel(string(p.Operation))
	return fmt.Sprintf("%s %s with %s", verb, p.AppName, p.Variant.Kind.DisplayName())
}

type Backend interface {
	Kind() model.ProviderKind
	Available() bool
	Installed(context.Context, model.Variant) bool
	Plan(model.Application, model.Distro, model.Variant, Operation) (Plan, error)
}

type UpdateChecker interface {
	UpdateAvailable(context.Context, model.Variant) bool
}

type VariantAvailability interface {
	AvailableVariant(model.Variant) bool
}

type DistroAvailability interface {
	AvailableFor(model.Distro) bool
}

type Runner interface {
	Run(context.Context, Step) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, step Step) ([]byte, error) {
	name := step.Executable
	args := append([]string(nil), step.Args...)
	if name == "ryomarket-portable" {
		resolved, err := localHelper(name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", step.Title, err)
		}
		name = resolved
	}
	if step.Elevated {
		program := name
		var err error
		if name == "ryomarket-nix" {
			program, err = trustedHelper(name)
		} else if !filepath.IsAbs(name) {
			program, err = exec.LookPath(name)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", step.Title, err)
		}
		args = append([]string{program}, args...)
		name = "pkexec"
	}
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s: %w", step.Title, err)
	}
	return out, nil
}

func Execute(ctx context.Context, runner Runner, plan Plan) (string, error) {
	var output strings.Builder
	for _, step := range plan.Steps {
		out, err := runner.Run(ctx, step)
		output.Write(out)
		if len(out) > 0 && out[len(out)-1] != '\n' {
			output.WriteByte('\n')
		}
		if err != nil {
			return output.String(), err
		}
	}
	return output.String(), nil
}

func localHelper(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(filepath.Dir(executable), name)
	info, err := os.Stat(candidate)
	if err != nil {
		return "", err
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("%s is not executable", candidate)
	}
	return candidate, nil
}

func trustedHelper(name string) (string, error) {
	candidates := []string{}
	if path, err := exec.LookPath(name); err == nil {
		candidates = append(candidates, path)
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), name))
	}
	candidates = append(candidates, filepath.Join("/usr/local/bin", name), filepath.Join("/run/current-system/sw/bin", name))
	seen := map[string]bool{}
	for _, path := range candidates {
		if seen[path] {
			continue
		}
		seen[path] = true
		resolved, err := trustedHelperPath(path)
		if err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("%s must be installed as a root-owned, non-writable system helper", name)
}

func trustedHelperPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("%s is not executable", resolved)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || info.Mode().Perm()&0o022 != 0 {
		return "", fmt.Errorf("%s must be root-owned and non-writable by group or others", resolved)
	}
	return resolved, nil
}
