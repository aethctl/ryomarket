package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThemeInheritanceAndFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	wall, shell, theme := themePaths()
	for _, p := range []string{wall, shell, theme} {
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, s string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(wall, `{"primary":"#abcdef","surface":"#121212","onPrimary":"#101010"}`)
	write(shell, `{"fontFamily":"Space Grotesk"}`)
	p := loadPalette()
	if p.accent != hex(0xabcdef) || p.bg != hex(0x121212) || uiTypeface() != "Space Grotesk" {
		t.Fatal("wallpaper or font not inherited")
	}
	write(shell, `{"themePalette":{"primary":"#ffcc00"}}`)
	if loadPalette().accent != hex(0xffcc00) {
		t.Fatal("named palette did not take precedence")
	}
	write(shell, `{invalid`)
	write(theme, `{"followWallpaper":false}`)
	if loadPalette().accent != hex(0xcebea6) {
		t.Fatal("fallback or wallpaper opt out ignored")
	}
}
