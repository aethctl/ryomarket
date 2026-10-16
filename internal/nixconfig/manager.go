package nixconfig

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var modulePath = "/etc/nixos/ryomarket.nix"
var configPath = "/etc/nixos/configuration.nix"
var flakePath = "/etc/nixos/flake.nix"
var effectiveUID = os.Geteuid
var systemHostname = os.Hostname
var rebuildSystem = defaultRebuildSystem

const moduleTemplate = "{ pkgs, ... }:\n\n{\n  environment.systemPackages = with pkgs; [\n  ];\n}\n"

func defaultRebuildSystem() error {
	binary := "/run/current-system/sw/bin/nixos-rebuild"
	if _, err := os.Stat(binary); err != nil {
		resolved, lookupErr := exec.LookPath("nixos-rebuild")
		if lookupErr != nil {
			return lookupErr
		}
		binary = resolved
	}
	args := []string{"switch"}
	if _, err := os.Stat(flakePath); err == nil {
		host, hostErr := systemHostname()
		if hostErr != nil {
			return hostErr
		}
		host = strings.TrimSpace(host)
		if host == "" {
			return errors.New("cannot determine NixOS flake configuration name from hostname")
		}
		args = append(args, "--flake", filepath.Dir(flakePath)+"#"+host)
	}
	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

var safePackage = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

type moduleSnapshot struct {
	exists bool
	data   []byte
}

func snapshotModule() (moduleSnapshot, error) {
	data, err := os.ReadFile(modulePath)
	if errors.Is(err, os.ErrNotExist) {
		return moduleSnapshot{}, nil
	}
	if err != nil {
		return moduleSnapshot{}, err
	}
	return moduleSnapshot{exists: true, data: data}, nil
}

func restoreModule(snapshot moduleSnapshot) error {
	if !snapshot.exists {
		if err := os.Remove(modulePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	tmp := modulePath + ".rollback"
	if err := os.WriteFile(tmp, snapshot.data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, modulePath)
}

func Apply(pkg string, add bool) error {
	snapshot, err := snapshotModule()
	if err != nil {
		return err
	}
	if err := mutate(pkg, add); err != nil {
		return err
	}
	if err := Rebuild(); err != nil {
		if restoreErr := restoreModule(snapshot); restoreErr != nil {
			return fmt.Errorf("rebuild failed: %v; rollback failed: %w", err, restoreErr)
		}
		if rollbackErr := Rebuild(); rollbackErr != nil {
			return fmt.Errorf("rebuild failed: %v; configuration restored but rollback rebuild failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("rebuild failed and the Ryomarket change was rolled back: %w", err)
	}
	return nil
}

func Setup() error {
	if effectiveUID() != 0 {
		return errors.New("ryomarket-nix must run as root")
	}

	configSnapshot, err := snapshotFile(configPath)
	if err != nil {
		return err
	}
	moduleSnapshot, err := snapshotFile(modulePath)
	if err != nil {
		return err
	}

	moduleCreated := false
	if !moduleSnapshot.exists {
		if err := os.WriteFile(modulePath, []byte(moduleTemplate), 0o644); err != nil {
			return err
		}
		moduleCreated = true
	}

	changed, err := ensureImport()
	if err != nil {
		_ = restoreFile(modulePath, moduleSnapshot)
		return err
	}
	if !changed && !moduleCreated && Supported() {
		return nil
	}

	if err := Rebuild(); err != nil {
		configRestoreErr := restoreFile(configPath, configSnapshot)
		moduleRestoreErr := restoreFile(modulePath, moduleSnapshot)
		if configRestoreErr != nil || moduleRestoreErr != nil {
			return fmt.Errorf("setup rebuild failed: %v; rollback failed: config=%v module=%v", err, configRestoreErr, moduleRestoreErr)
		}
		if rollbackErr := Rebuild(); rollbackErr != nil {
			return fmt.Errorf("setup rebuild failed: %v; files restored but rollback rebuild failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("setup rebuild failed and was rolled back: %w", err)
	}
	return nil
}

type fileSnapshot struct {
	exists bool
	data   []byte
}

func snapshotFile(path string) (fileSnapshot, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileSnapshot{}, nil
	}
	if err != nil {
		return fileSnapshot{}, err
	}
	return fileSnapshot{exists: true, data: data}, nil
}

func restoreFile(path string, snapshot fileSnapshot) error {
	if !snapshot.exists {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	tmp := path + ".rollback"
	if err := os.WriteFile(tmp, snapshot.data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ensureImport() (bool, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}
	text := string(data)
	if strings.Contains(text, "./ryomarket.nix") || strings.Contains(text, "ryomarket.nix") {
		return false, nil
	}

	lines := strings.Split(text, "\n")
	importsStart := -1
	indent := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "imports") && strings.Contains(trimmed, "=") && strings.Contains(trimmed, "[") {
			if strings.Contains(trimmed, "];") {
				return false, fmt.Errorf("%s uses an inline imports list Ryomarket will not rewrite automatically", configPath)
			}
			importsStart = i
			indent = line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			break
		}
	}
	if importsStart == -1 {
		return false, fmt.Errorf("%s has no imports list Ryomarket can safely extend", configPath)
	}

	importsEnd := -1
	for i := importsStart; i < len(lines); i++ {
		if strings.Contains(lines[i], "];") {
			importsEnd = i
			break
		}
	}
	if importsEnd == -1 {
		return false, fmt.Errorf("%s has an imports list Ryomarket could not parse safely", configPath)
	}

	entryIndent := indent + "  "
	entry := entryIndent + "./ryomarket.nix"
	lines = append(lines[:importsEnd], append([]string{entry}, lines[importsEnd:]...)...)
	tmp := configPath + ".ryomarket.tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, configPath); err != nil {
		return false, err
	}
	return true, nil
}

func Supported() bool {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	text := string(b)
	return strings.Contains(text, "./ryomarket.nix") || strings.Contains(text, "ryomarket.nix")
}

func Packages() ([]string, error) {
	f, err := os.Open(modulePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var packages []string
	s := bufio.NewScanner(f)
	inside := false
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.Contains(line, "environment.systemPackages") && strings.Contains(line, "[") {
			inside = true
			continue
		}
		if inside && line == "];" {
			break
		}
		if inside && line != "" && !strings.HasPrefix(line, "#") {
			pkg := strings.Fields(line)[0]
			if !safePackage.MatchString(pkg) {
				return nil, fmt.Errorf("%s contains an unmanaged package expression", modulePath)
			}
			packages = append(packages, pkg)
		}
	}
	return packages, s.Err()
}

func Has(pkg string) bool {
	packages, err := Packages()
	if err != nil {
		return false
	}
	for _, item := range packages {
		if item == pkg {
			return true
		}
	}
	return false
}

func Add(pkg string) error {
	return mutate(pkg, true)
}

func Remove(pkg string) error {
	return mutate(pkg, false)
}

func mutate(pkg string, add bool) error {
	if !safePackage.MatchString(pkg) {
		return fmt.Errorf("invalid Nix package attribute %q", pkg)
	}
	if effectiveUID() != 0 {
		return errors.New("ryomarket-nix must run as root")
	}
	if !Supported() {
		return fmt.Errorf("%s must import ./ryomarket.nix before declarative installs are enabled", configPath)
	}
	packages, err := Packages()
	if err != nil {
		return err
	}
	set := map[string]bool{}
	for _, item := range packages {
		set[item] = true
	}
	if add {
		set[pkg] = true
	} else {
		delete(set, pkg)
	}
	packages = packages[:0]
	for item := range set {
		packages = append(packages, item)
	}
	sort.Strings(packages)
	return writeModule(packages)
}

func writeModule(packages []string) error {
	var b strings.Builder
	b.WriteString("{ pkgs, ... }:\n\n{\n  environment.systemPackages = with pkgs; [\n")
	for _, pkg := range packages {
		b.WriteString("    ")
		b.WriteString(pkg)
		b.WriteByte('\n')
	}
	b.WriteString("  ];\n}\n")
	if err := os.MkdirAll(filepath.Dir(modulePath), 0o755); err != nil {
		return err
	}
	tmp := modulePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, modulePath)
}

func Rebuild() error {
	if effectiveUID() != 0 {
		return errors.New("ryomarket-nix must run as root")
	}
	return rebuildSystem()
}
