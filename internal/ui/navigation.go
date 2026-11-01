package ui

import (
	"image"
	"math"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var navigationIcons = func() map[string]*widget.Icon {
	out := map[string]*widget.Icon{}
	for name, data := range map[string][]byte{
		viewDiscover: icons.ActionExplore, viewBrowse: icons.ActionViewModule,
		viewInstalled: icons.FileFolder, viewUpdates: icons.NavigationRefresh, viewPreferences: icons.ActionSettings,
		"search": icons.ActionSearch, "collapse": icons.NavigationChevronLeft, "expand": icons.NavigationChevronRight,
	} {
		icon, err := widget.NewIcon(data)
		if err == nil {
			out[name] = icon
		}
	}
	return out
}()

func (a *App) navigation(gtx layout.Context, compact bool) layout.Dimensions {
	fill(gtx, colors.rail)
	return layout.Inset{Top: unit.Dp(30), Bottom: unit.Dp(20), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.brand(gtx, compact) }),
			layout.Rigid(layout.Spacer{Height: unit.Dp(28)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if compact {
					if a.searchButton.Clicked(gtx) {
						a.collapsed = false
						a.navigationExpanded = true
						a.navigate(viewBrowse)
						gtx.Execute(key.FocusCmd{Tag: &a.query})
					}
					return a.railAction(gtx, &a.searchButton, "search", "Search", true, false)
				}
				return a.searchBox(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(28)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				entries := []struct{ id, name string }{{viewDiscover, "Discover"}, {viewBrowse, "Browse all"}, {viewInstalled, "Installed"}, {viewUpdates, "Updates"}}
				return a.navigationList.Layout(gtx, len(entries), func(gtx layout.Context, index int) layout.Dimensions {
					entry := entries[index]
					click := a.navButtons[entry.id]
					if click.Clicked(gtx) {
						a.navigate(entry.id)
					}
					return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.railAction(gtx, click, entry.id, entry.name, compact, a.view == entry.id)
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.preferencesButton.Clicked(gtx) {
					a.navigate(viewPreferences)
				}
				return a.railAction(gtx, &a.preferencesButton, viewPreferences, "Preferences", compact, a.view == viewPreferences)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.collapseButton.Clicked(gtx) {
					a.collapsed = !compact
					a.navigationExpanded = compact
				}
				name, id := "Collapse", "collapse"
				if compact {
					name, id = "Expand", "expand"
				}
				return a.railAction(gtx, &a.collapseButton, id, name, compact, false)
			}),
		)
	})
}

func (a *App) railAction(gtx layout.Context, click *widget.Clickable, id, name string, compact, selected bool) layout.Dimensions {
	fg, bg := colors.subtle, colors.rail
	if selected {
		fg = colors.accent
		bg = blendColor(colors.rail, colors.accent, 0.12)
	} else if click.Hovered() || gtx.Focused(click) {
		fg = colors.bright
		bg = colors.surface2
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(name).Add(gtx.Ops)
		semantic.SelectedOp(selected).Add(gtx.Ops)
		return roundedBackground(gtx, bg, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
			in := layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(12), Right: unit.Dp(10)}
			if compact {
				in = layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10)}
			}
			return in.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				icon := func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(21)), gtx.Dp(unit.Dp(21)))
					gtx.Constraints.Max = gtx.Constraints.Min
					return navigationIcons[id].Layout(gtx, fg)
				}
				if compact {
					return layout.Center.Layout(gtx, icon)
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, layout.Rigid(icon), layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout), layout.Flexed(1, mediumLabel(a.theme, unit.Sp(14), name, fg).Layout))
			})
		})
	})
}

func (a *App) shortcuts(gtx layout.Context) {
	for {
		e, ok := gtx.Event(key.Filter{Name: "1", Required: key.ModCtrl}, key.Filter{Name: "2", Required: key.ModCtrl}, key.Filter{Name: "3", Required: key.ModCtrl}, key.Filter{Name: "4", Required: key.ModCtrl}, key.Filter{Name: "5", Required: key.ModCtrl}, key.Filter{Name: "F", Required: key.ModCtrl}, key.Filter{Name: "F", Required: key.ModCtrl | key.ModShift}, key.Filter{Name: key.NameReturn, Required: key.ModCtrl}, key.Filter{Name: ",", Required: key.ModCtrl}, key.Filter{Name: "\\", Required: key.ModCtrl}, key.Filter{Name: key.NameEscape}, key.Filter{Name: key.NameLeftArrow, Required: key.ModAlt}, key.Filter{Name: key.NameRightArrow, Required: key.ModAlt})
		if !ok {
			break
		}
		event, ok := e.(key.Event)
		if !ok || event.State != key.Press {
			continue
		}
		switch event.Name {
		case "1":
			a.navigate(viewDiscover)
		case "2":
			a.navigate(viewBrowse)
		case "3":
			a.navigate(viewInstalled)
		case "4":
			a.navigate(viewUpdates)
		case "5":
			a.navigate(viewBrowse)
		case key.NameReturn:
			if a.view == viewWelcome {
				a.advanceOnboarding()
			} else if a.view == viewBrowse {
				a.ensureDiscoverCache()
				if len(a.discoverApps) > 0 {
					a.openDetail(a.discoverApps[0])
				}
			} else if a.view == viewDiscover {
				if apps := a.featuredApplications(); len(apps) > 0 {
					a.openDetail(apps[a.featureIndex%len(apps)])
				}
			}
		case "F":
			if event.Modifiers.Contain(key.ModShift) {
				if a.view != viewBrowse || a.selectedID != "" {
					a.navigate(viewBrowse)
				}
				a.sourcesOpen = !a.sourcesOpen
			} else {
				a.collapsed = false
				a.navigationExpanded = true
				a.navigate(viewBrowse)
				gtx.Execute(key.FocusCmd{Tag: &a.query})
			}
		case ",":
			a.navigate(viewPreferences)
		case "\\":
			compact := a.collapsed || (!a.navigationExpanded && gtx.Constraints.Max.X < gtx.Dp(unit.Dp(1050)))
			a.collapsed = !compact
			a.navigationExpanded = compact
		case key.NameEscape:
			if a.view == viewWelcome {
				a.skipOnboarding()
			}
			a.sourcesOpen = false
			if a.selectedID != "" {
				a.selectedID = ""
				a.pending = nil
			}
		case key.NameLeftArrow:
			if a.selectedID != "" {
				a.galleryIndex = max(0, a.galleryIndex-1)
			} else if a.view == viewDiscover {
				a.changeFeature(-1)
			}
		case key.NameRightArrow:
			if a.selectedID != "" {
				a.galleryIndex++
			} else if a.view == viewDiscover {
				a.changeFeature(1)
			}
		}
	}
}

func (a *App) navigationMotion(gtx layout.Context, content layout.Widget) layout.Dimensions {
	if a.reducedMotion.Value || a.motionStart.IsZero() {
		return content(gtx)
	}
	t := gtx.Now.Sub(a.motionStart).Seconds() / 0.28
	if t >= 1 {
		a.motionStart = time.Time{}
		return content(gtx)
	}
	if t < 0 {
		t = 0
	}
	// A critically damped entrance settles without an idle animation loop.
	y := int(12 * (1 + 12*t) * math.Exp(-12*t))
	defer op.Offset(image.Pt(0, y)).Push(gtx.Ops).Pop()
	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second / 60)})
	return content(gtx)
}
