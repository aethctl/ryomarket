package nixconfig

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestSafePackageAttribute(t *testing.T) {
	for _, value := range []string{"obs-studio", "godot_4", "libreoffice-qt", "foo.bar", "qt6Packages.kdenlive"} {
		if !safePackage.MatchString(value) {
			t.Fatalf("expected %q to be accepted", value)
		}
	}
}

func TestUnsafePackageAttribute(t *testing.T) {
	for _, value := range []string{"", "../evil", "foo bar", "foo;bar", "foo\nbar", "{ pkgs }"} {
		if safePackage.MatchString(value) {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestApplyRollsBackFailedRebuild(t *testing.T) {
	temp := t.TempDir()
	oldModulePath, oldConfigPath := modulePath, configPath
	oldUID, oldRebuild := effectiveUID, rebuildSystem
	defer func() {
		modulePath, configPath = oldModulePath, oldConfigPath
		effectiveUID, rebuildSystem = oldUID, oldRebuild
	}()

	modulePath = temp + "/ryomarket.nix"
	configPath = temp + "/configuration.nix"
	effectiveUID = func() int { return 0 }
	if err := os.WriteFile(configPath, []byte("{ ... }: { imports = [ ./ryomarket.nix ]; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := []byte("{ pkgs, ... }:\n\n{\n  environment.systemPackages = with pkgs; [\n    firefox\n  ];\n}\n")
	if err := os.WriteFile(modulePath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	rebuildSystem = func() error {
		calls++
		if calls == 1 {
			return errors.New("broken rebuild")
		}
		return nil
	}

	if err := Apply("obs-studio", true); err == nil {
		t.Fatal("expected rebuild failure")
	}
	got, err := os.ReadFile(modulePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("module was not restored:\n%s", got)
	}
	if calls != 2 {
		t.Fatalf("expected failed rebuild plus rollback rebuild, got %d calls", calls)
	}
}

func TestSetupAddsManagedModuleAndImport(t *testing.T) {
	temp := t.TempDir()
	oldModulePath, oldConfigPath := modulePath, configPath
	oldUID, oldRebuild := effectiveUID, rebuildSystem
	defer func() {
		modulePath, configPath = oldModulePath, oldConfigPath
		effectiveUID, rebuildSystem = oldUID, oldRebuild
	}()

	modulePath = temp + "/ryomarket.nix"
	configPath = temp + "/configuration.nix"
	effectiveUID = func() int { return 0 }
	original := "{ ... }:\n{\n  imports = [\n    ./hardware-configuration.nix\n  ];\n}\n"
	if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	rebuildSystem = func() error {
		calls++
		return nil
	}

	if err := Setup(); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "./ryomarket.nix") {
		t.Fatalf("setup did not add import:\n%s", config)
	}
	module, err := os.ReadFile(modulePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(module) != moduleTemplate {
		t.Fatalf("unexpected module contents:\n%s", module)
	}
	if calls != 1 {
		t.Fatalf("expected one rebuild, got %d", calls)
	}

	if err := Setup(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("idempotent setup should not rebuild again, got %d calls", calls)
	}
}

func TestSetupRollsBackFailedRebuild(t *testing.T) {
	temp := t.TempDir()
	oldModulePath, oldConfigPath := modulePath, configPath
	oldUID, oldRebuild := effectiveUID, rebuildSystem
	defer func() {
		modulePath, configPath = oldModulePath, oldConfigPath
		effectiveUID, rebuildSystem = oldUID, oldRebuild
	}()

	modulePath = temp + "/ryomarket.nix"
	configPath = temp + "/configuration.nix"
	effectiveUID = func() int { return 0 }
	original := []byte("{ ... }:\n{\n  imports = [\n    ./hardware-configuration.nix\n  ];\n}\n")
	if err := os.WriteFile(configPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	rebuildSystem = func() error {
		calls++
		if calls == 1 {
			return errors.New("broken setup")
		}
		return nil
	}

	if err := Setup(); err == nil {
		t.Fatal("expected setup failure")
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("configuration was not restored:\n%s", got)
	}
	if _, err := os.Stat(modulePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("managed module should have been removed after rollback, err=%v", err)
	}
	if calls != 2 {
		t.Fatalf("expected failed rebuild plus rollback rebuild, got %d", calls)
	}
}

func TestSetupRefusesInlineImportsList(t *testing.T) {
	temp := t.TempDir()
	oldModulePath, oldConfigPath := modulePath, configPath
	oldUID, oldRebuild := effectiveUID, rebuildSystem
	defer func() {
		modulePath, configPath = oldModulePath, oldConfigPath
		effectiveUID, rebuildSystem = oldUID, oldRebuild
	}()

	modulePath = temp + "/ryomarket.nix"
	configPath = temp + "/configuration.nix"
	effectiveUID = func() int { return 0 }
	if err := os.WriteFile(configPath, []byte("{ ... }: { imports = [ ./hardware.nix ]; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rebuildSystem = func() error { return nil }

	if err := Setup(); err == nil {
		t.Fatal("expected setup to refuse inline imports list")
	}
	if _, err := os.Stat(modulePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("managed module should not remain after refused setup, err=%v", err)
	}
}
