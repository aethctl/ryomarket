package ui

import (
	"fmt"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/Ryoku-dev/ryomarket/internal/model"
)

func methodName(kind model.ProviderKind) string {
	switch kind {
	case model.ProviderNixFlake:
		return "Nix flake"
	case model.ProviderNixProfile:
		return "Nix profile"
	case model.ProviderNixConfig:
		return "Nix configuration"
	case model.ProviderFlatpak:
		return "Flatpak"
	case model.ProviderAppImage:
		return "AppImage"
	case "":
		return "Automatic"
	default:
		return kind.DisplayName()
	}
}

func methodDescription(kind model.ProviderKind) string {
	switch kind {
	case model.ProviderNixFlake:
		return "Install from a pinned flake into a separate RyoMarket profile."
	case model.ProviderNixProfile:
		return "Install in your user profile without rebuilding NixOS."
	case model.ProviderNixConfig:
		return "Add to your NixOS configuration. Rebuild when you choose."
	case model.ProviderFlatpak:
		return "Isolated applications from Flathub."
	case model.ProviderAppImage:
		return "Portable applications managed for your user."
	case "":
		return "Use a compatible recommended source for each application."
	default:
		return "Use this application's supported installation source."
	}
}

func (a *App) methodAvailable(variant model.Variant) bool {
	if variant.Kind == model.ProviderNixConfig {
		return a.selected == model.DistroNixOS
	}
	return a.service != nil && a.service.VariantAvailable(a.selected, variant)
}

func (a *App) preferenceAvailable(kind model.ProviderKind) bool {
	if kind == "" {
		return true
	}
	if kind == model.ProviderNixConfig {
		return a.selected == model.DistroNixOS
	}
	if a.service == nil {
		return false
	}
	if kind == model.ProviderAppImage || kind == model.ProviderNixFlake {
		_, ok := a.service.Backend(kind)
		return ok
	}
	return a.service.VariantAvailable(a.selected, model.Variant{Kind: kind})
}

func (a *App) methodControl(item model.Application, index int) *widget.Clickable {
	if a.providerButtons == nil {
		a.providerButtons = map[string]*widget.Clickable{}
	}
	key := fmt.Sprintf("%s:%s:%d", item.ID, a.selected, index)
	click := a.providerButtons[key]
	if click == nil {
		click = new(widget.Clickable)
		a.providerButtons[key] = click
	}
	return click
}

func (a *App) updateMethodSelection(gtx layout.Context, item model.Application) {
	for index, variant := range item.Variants[a.selected] {
		if a.methodControl(item, index).Clicked(gtx) && !a.busy && a.methodAvailable(variant) {
			a.variantSelection[item.ID] = index
			a.pending = nil
			a.showConfig = false
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (a *App) methodOption(gtx layout.Context, item model.Application, index int, selected bool) layout.Dimensions {
	variant := item.Variants[a.selected][index]
	available := a.methodAvailable(variant)
	click := a.methodControl(item, index)
	if !available || a.busy {
		gtx = gtx.Disabled()
	}
	bg, border := colors.surface2, colors.line
	if selected {
		bg = blendColor(bg, colors.accent, 0.08)
		border = colors.accent
	}
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(methodName(variant.Kind) + ". " + methodDescription(variant.Kind)).Add(gtx.Ops)
			semantic.SelectedOp(selected).Add(gtx.Ops)
			semantic.EnabledOp(available).Add(gtx.Ops)
			return widget.Border{Color: border, CornerRadius: unit.Dp(8), Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return roundedBackground(gtx, bg, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(13)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						detail := methodDescription(variant.Kind)
						if !available {
							detail = "Not available on this system."
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(heading(a.theme, unit.Sp(14), methodName(variant.Kind), chooseColor(available, colors.bright, colors.subtle)).Layout),
							layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
							layout.Rigid(label(a.theme, unit.Sp(12), detail, colors.subtle).Layout),
						)
					})
				})
			})
		})
	})
}

func preferredFallbackText(preferred model.ProviderKind, actual model.ProviderKind) string {
	if preferred == "" || preferred == actual {
		return ""
	}
	return methodName(preferred) + " is unavailable for this application. Using " + methodName(actual) + " instead."
}
