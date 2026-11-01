package ui

import (
	"fmt"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"github.com/Ryoku-dev/ryomarket/internal/provider"
	"github.com/Ryoku-dev/ryomarket/internal/service"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (a *App) management(gtx layout.Context, updatesOnly bool) layout.Dimensions {
	items := a.installed
	if updatesOnly {
		items = nil
		for _, update := range a.updates {
			items = append(items, update.Installed)
		}
	}
	if len(items) == 0 {
		if a.scanBusy {
			return a.empty(gtx, "Checking applications", "Reading installed sources without changing your packages.")
		}
		return a.empty(gtx, choose(updatesOnly, "No updates found", "No managed applications found"), "Applications managed through supported sources appear here.")
	}
	var unique []service.Installed
	seen := map[string]bool{}
	for _, entry := range items {
		if !seen[entry.App.ID] {
			seen[entry.App.ID] = true
			unique = append(unique, entry)
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if a.managementRefresh.Clicked(gtx) && !a.scanBusy {
				a.startScan(true)
			}
			return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, button(a.theme, &a.managementRefresh, choose(a.scanBusy, "Checking…", "Check for updates"), false).Layout)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.list.Layout(gtx, len(unique), func(gtx layout.Context, index int) layout.Dimensions { return a.managementRow(gtx, unique[index]) })
		}),
	)
}

func (a *App) managementControl(id, action string) *widget.Clickable {
	if a.managementButtons == nil {
		a.managementButtons = map[string]*widget.Clickable{}
	}
	key := id + ":" + action
	click := a.managementButtons[key]
	if click == nil {
		click = new(widget.Clickable)
		a.managementButtons[key] = click
	}
	return click
}

func (a *App) managementRow(gtx layout.Context, entry service.Installed) layout.Dimensions {
	item, index := entry.App, entry.VariantIndex
	variants := item.Variants[a.selected]
	if index < 0 || index >= len(variants) {
		return layout.Dimensions{}
	}
	variant := variants[index]
	manage := a.managementControl(item.ID, "manage")
	remove := a.managementControl(item.ID, "remove")
	open := a.managementControl(item.ID, "open")
	update := a.managementControl(item.ID, "update")
	if manage.Clicked(gtx) {
		a.openDetail(item)
	}
	if remove.Clicked(gtx) {
		a.prepareManagedAction(entry, provider.Remove)
	}
	var pendingUpdate *service.Installed
	for _, u := range a.updates {
		if u.App.ID == item.ID {
			copy := u.Installed
			pendingUpdate = &copy
			break
		}
	}
	if update.Clicked(gtx) && pendingUpdate != nil {
		a.prepareManagedAction(*pendingUpdate, provider.Update)
	}
	command, canOpen := launchCommand(item, variant)
	if open.Clicked(gtx) && canOpen {
		a.launchApplication(item.Name, command)
	}
	return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return roundedBackground(gtx, colors.surface2, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				identity := func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.applicationIcon(gtx, item, 48) }), layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(heading(a.theme, unit.Sp(16), item.Name, colors.bright).Layout),
								layout.Rigid(label(a.theme, unit.Sp(12), methodName(variant.Kind)+choose(pendingUpdate != nil, " · Update available", " · Installed"), colors.subtle).Layout),
							)
						}),
					)
				}
				controls := func(gtx layout.Context) layout.Dimensions {
					children := []layout.FlexChild{}
					if canOpen {
						children = append(children, layout.Rigid(button(a.theme, open, "Open", false).Layout), layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
					}
					if pendingUpdate != nil {
						children = append(children, layout.Rigid(button(a.theme, update, "Update", true).Layout), layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
					}
					children = append(children, layout.Rigid(button(a.theme, manage, "Manage", false).Layout))
					if variant.Kind != model.ProviderNixConfig {
						children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout), layout.Rigid(button(a.theme, remove, "Remove", false).Layout))
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
				}
				if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(800)) {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(identity), layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout), layout.Rigid(controls))
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, layout.Flexed(1, identity), layout.Rigid(controls))
			})
		})
	})
}

func (a *App) prepareManagedAction(entry service.Installed, op provider.Operation) {
	a.openDetail(entry.App)
	a.variantSelection[entry.App.ID] = entry.VariantIndex
	variants := entry.App.Variants[a.selected]
	if entry.VariantIndex < 0 || entry.VariantIndex >= len(variants) {
		return
	}
	if variants[entry.VariantIndex].Kind == model.ProviderNixConfig {
		a.showConfig = true
		return
	}
	plan, err := a.service.Plan(entry.App, a.selected, entry.VariantIndex, op)
	if err != nil {
		a.status, a.statusBad = err.Error(), true
		return
	}
	a.pending = &plan
}

func launchCommand(item model.Application, variant model.Variant) ([]string, bool) {
	if variant.Kind == model.ProviderFlatpak {
		if path, err := exec.LookPath("flatpak"); err == nil && variant.Package != "" {
			return []string{path, "run", variant.Package}, true
		}
		return nil, false
	}
	gio, err := exec.LookPath("gio")
	if err != nil {
		return nil, false
	}
	home, _ := os.UserHomeDir()
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	roots := []string{dataHome, filepath.Join(home, ".nix-profile", "share")}
	roots = append(roots, filepath.SplitList(os.Getenv("XDG_DATA_DIRS"))...)
	ids := []string{item.ID, flatpakID(item), variant.Package, "ryomarket-" + item.ID}
	if variant.Kind == model.ProviderNixFlake {
		ids = append([]string{"dev.ryoku.ryomarket." + strings.TrimPrefix(item.ID, "ryoku:")}, ids...)
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		for _, id := range ids {
			if !safeDesktopID(id) {
				continue
			}
			path := filepath.Join(root, "applications", id+".desktop")
			if desktopApplication(path) {
				return []string{gio, "launch", path}, true
			}
		}
	}
	return nil, false
}

func safeDesktopID(value string) bool {
	if value == "" || strings.Contains(value, "..") {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func desktopApplication(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > 256<<10 {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	entry := false
	application, hasExec := false, false
	for _, line := range strings.Split(string(data), `
`) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			entry = line == "[Desktop Entry]"
			continue
		}
		if !entry {
			continue
		}
		if line == "Hidden=true" {
			return false
		}
		if line == "Type=Application" {
			application = true
		}
		if strings.HasPrefix(line, "Exec=") && strings.TrimSpace(strings.TrimPrefix(line, "Exec=")) != "" {
			hasExec = true
		}
	}
	return application && hasExec
}

func (a *App) launchApplication(name string, args []string) {
	if len(args) == 0 {
		return
	}
	command := exec.Command(args[0], args[1:]...)
	if err := command.Start(); err != nil {
		a.status, a.statusBad = "Could not open "+name+". Try your desktop launcher.", true
		return
	}
	a.status, a.statusBad = "Launch requested for "+name+".", false
	go func() {
		if err := command.Wait(); err != nil {
			select {
			case a.launchErrors <- fmt.Errorf("Could not open %s. Try your desktop launcher.", name):
			default:
			}
			if a.win != nil {
				a.win.Invalidate()
			}
		}
	}()
}
