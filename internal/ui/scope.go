package ui

import (
	"image/color"
	"strconv"

	"gioui.org/font"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/Ryoku-dev/ryomarket/internal/model"
)

const sourceAll = "all"

type sourceFilter struct {
	ID    string
	Label string
	Kinds []model.ProviderKind
}

var sourceFilters = []sourceFilter{
	{ID: sourceAll, Label: "All"},
	{ID: "nix-flake", Label: "Nix flake", Kinds: []model.ProviderKind{model.ProviderNixFlake}},
	{ID: "nix-profile", Label: "Nix", Kinds: []model.ProviderKind{model.ProviderNixProfile}},
	{ID: "nix-config", Label: "Nix Configuration", Kinds: []model.ProviderKind{model.ProviderNixConfig}},
	{ID: "flatpak", Label: "Flatpak", Kinds: []model.ProviderKind{model.ProviderFlatpak}},
	{ID: "appimage", Label: "AppImage", Kinds: []model.ProviderKind{model.ProviderAppImage}},
	{ID: "upstream", Label: "Direct download", Kinds: []model.ProviderKind{model.ProviderUpstream}},
	{ID: "dnf", Label: "DNF", Kinds: []model.ProviderKind{model.ProviderDNF}},
	{ID: "pacman", Label: "Pacman", Kinds: []model.ProviderKind{model.ProviderPacman}},
	{ID: "xbps", Label: "XBPS", Kinds: []model.ProviderKind{model.ProviderXBPS}},
}

func buildSourceCounts(total int, counts map[model.ProviderKind]int) map[string]int {
	out := make(map[string]int, len(sourceFilters))
	out[sourceAll] = total
	for _, source := range sourceFilters {
		if source.ID == sourceAll {
			continue
		}
		count := 0
		for _, kind := range source.Kinds {
			count += counts[kind]
		}
		out[source.ID] = count
	}
	return out
}

func (a *App) refreshCatalogStats() {
	total, counts := a.catalog.ProviderCountsFor(a.selected, a.browseProviders())
	a.catalogTotal = total
	a.sourceCounts = buildSourceCounts(total, counts)
}

func (a *App) sourceCount(id string) int {
	if a.sourceCounts == nil {
		return 0
	}
	return a.sourceCounts[id]
}

func sourceKinds(id string) []model.ProviderKind {
	for _, source := range sourceFilters {
		if source.ID == id {
			return source.Kinds
		}
	}
	return nil
}

func scopeSummary(app model.Application) string {
	var scopes [6]bool
	for distro, variants := range app.Variants {
		for _, variant := range variants {
			switch variant.Kind {
			case model.ProviderFlatpak, model.ProviderAppImage, model.ProviderUpstream:
				scopes[0] = true
			case model.ProviderNixFlake, model.ProviderNixProfile:
				if distro == model.DistroNixOS {
					scopes[5] = true
				} else {
					scopes[1] = true
				}
			case model.ProviderDNF:
				scopes[2] = true
			case model.ProviderPacman:
				scopes[3] = true
			case model.ProviderXBPS:
				scopes[4] = true
			case model.ProviderNixConfig:
				scopes[5] = true
			}
		}
	}
	labels := [...]string{"UNIVERSAL", "UNIVERSAL*", "FEDORA", "ARCH", "VOID", "NIX"}
	first := ""
	count := 0
	for i, present := range scopes {
		if !present {
			continue
		}
		if first == "" {
			first = labels[i]
		}
		count++
	}
	if count == 0 {
		return "UNAVAILABLE"
	}
	if count == 1 {
		return first
	}
	return first + " +" + strconv.Itoa(count-1)
}

func (a *App) sourceRail(gtx layout.Context) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(sourceFilters))
	for _, source := range a.visibleSources() {
		source := source
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.sourceRailItem(gtx, source)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) sourceRailItem(gtx layout.Context, source sourceFilter) layout.Dimensions {
	click := a.sourceButtons[source.ID]
	if click == nil {
		click = new(widget.Clickable)
		a.sourceButtons[source.ID] = click
	}
	if click.Clicked(gtx) {
		a.source = source.ID
		a.category = "All"
		a.view = viewBrowse
		a.selectedID = ""
		a.pending = nil
		a.list.Position = layout.Position{}
	}

	active := a.source == source.ID && a.view == viewBrowse
	count := a.sourceCount(source.ID)

	fg := colors.cream
	meta := colors.dim
	bg := color.NRGBA{}
	if active {
		bg = colors.accent
		fg = colors.accentInk
		meta = colors.accentInk
	}

	return layout.Inset{Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			content := func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(10), Right: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							l := mediumLabel(a.theme, unit.Sp(13), source.Label, fg)
							if active {
								l.Color = colors.accentInk
								l.Font.Weight = font.Bold
							}
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								l := mediumLabel(a.theme, unit.Sp(11), formatCount(count), meta)
								if active {
									l.Color = colors.accentInk
									l.Font.Weight = font.SemiBold
								}
								return l.Layout(gtx)
							})
						}),
					)
				})
			}
			if active {
				return roundedBackground(gtx, bg, unit.Dp(6), content)
			}
			if click.Hovered() {
				return roundedBackground(gtx, colors.surface, unit.Dp(6), content)
			}
			return content(gtx)
		})
	})
}

func (a *App) systemCard(gtx layout.Context) layout.Dimensions {
	name := a.host.Distro.DisplayName()
	if a.host.Distro == model.DistroUnknown {
		name = "Unsupported host"
	}
	state := formatCount(a.catalogTotal) + " apps"
	if a.catalogLoading {
		state = "Catalogue syncing"
	}
	return boxed(gtx, colors.surface, colors.line, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(label(a.theme, unit.Sp(8), "THIS SYSTEM", colors.dim).Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(mediumLabel(a.theme, unit.Sp(12), name, colors.bright).Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
			layout.Rigid(label(a.theme, unit.Sp(9), state, colors.dim).Layout),
		)
	})
}

func (a *App) compactActionButton(gtx layout.Context, text string, primary bool) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = 0
			style := button(a.theme, &a.actionButton, text, primary)
			style.Inset = layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(24), Right: unit.Dp(24)}
			return style.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
	)
}

func providerMatchesHost(d model.Distro, kind model.ProviderKind) bool {
	switch kind {
	case model.ProviderDNF:
		return d == model.DistroFedora
	case model.ProviderPacman:
		return d == model.DistroArch || d == model.DistroCachyOS
	case model.ProviderXBPS:
		return d == model.DistroVoid
	case model.ProviderNixFlake, model.ProviderNixConfig:
		return d == model.DistroNixOS
	default:
		return kind == model.ProviderNixProfile || kind == model.ProviderFlatpak || kind == model.ProviderAppImage || kind == model.ProviderUpstream
	}
}

func (a *App) visibleSources() []sourceFilter {
	out := []sourceFilter{sourceFilters[0]}
	for _, source := range sourceFilters[1:] {
		kind := source.Kinds[0]
		if !providerMatchesHost(a.selected, kind) {
			continue
		}
		if (kind == model.ProviderNixConfig || kind == model.ProviderUpstream) && !a.advanced.Value {
			continue
		}
		if _, ok := a.service.Backend(kind); !ok && kind != model.ProviderNixConfig {
			continue
		}
		if kind != model.ProviderNixFlake && kind != model.ProviderNixConfig && kind != model.ProviderAppImage && kind != model.ProviderUpstream && !a.service.VariantAvailable(a.selected, model.Variant{Kind: kind}) {
			continue
		}
		out = append(out, source)
	}
	return out
}

func (a *App) browseProviders() []model.ProviderKind {
	var kinds []model.ProviderKind
	for _, source := range a.visibleSources()[1:] {
		kinds = append(kinds, source.Kinds...)
	}
	if len(kinds) == 0 {
		return []model.ProviderKind{"unavailable"}
	}
	return kinds
}
