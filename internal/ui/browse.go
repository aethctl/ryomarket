package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"strings"
)

func (a *App) browse(gtx layout.Context) layout.Dimensions {
	if a.query.Text() != a.lastQuery {
		a.lastQuery = a.query.Text()
		a.list.Position = layout.Position{}
	}
	a.ensureDiscoverCache()
	return layout.Stack{Alignment: layout.NE}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(a.categoryControls),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(a.discoverApps) == 0 {
						return a.empty(gtx, "No matches", "Try a different name, category or source.")
					}
					return a.appGrid(gtx, a.discoverApps)
				}),
			)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if !a.sourcesOpen {
				return layout.Dimensions{}
			}
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Stack{Alignment: layout.NE}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					if a.sourceDismiss.Clicked(gtx) {
						a.sourcesOpen = false
					}
					return a.sourceDismiss.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Max} })
				}),
				layout.Stacked(a.sourcePopover),
			)
		}),
	)
}

func (a *App) sourcePopover(gtx layout.Context) layout.Dimensions {
	width := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(340)))
	gtx.Constraints.Min.X = width
	gtx.Constraints.Max.X = width
	gtx.Constraints.Min.Y = 0
	a.sourceSurface.Clicked(gtx)
	return a.sourceSurface.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widget.Border{Color: colors.strong, Width: unit.Dp(1), CornerRadius: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return roundedBackground(gtx, colors.raised, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(18)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(heading(a.theme, unit.Sp(16), "Installation sources", colors.bright).Layout),
						layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
						layout.Rigid(a.sourceRail),
						layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if a.advanced.Update(gtx) {
								a.source = sourceAll
								a.invalidateDiscoverCache()
								a.refreshCatalogStats()
							}
							style := material.CheckBox(a.theme, &a.advanced, "Advanced sources")
							style.TextSize = unit.Sp(13)
							return style.Layout(gtx)
						}),
						layout.Rigid(label(a.theme, unit.Sp(11), "Compatible with this system. Esc to close.", colors.subtle).Layout),
					)
				})
			})
		})
	})
}

func (a *App) categoryControls(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				cats := a.discoverCats
				if !a.moreCategories {
					var selected []string
					for _, wanted := range []string{"All", "Development", "Graphics", "Gaming"} {
						for _, cat := range cats {
							if strings.EqualFold(cat, wanted) {
								selected = append(selected, cat)
								break
							}
						}
					}
					cats = selected
				}
				gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(38))
				return a.categoryList.Layout(gtx, len(cats), func(gtx layout.Context, i int) layout.Dimensions {
					cat := cats[i]
					if a.categoryButtons[cat] == nil {
						a.categoryButtons[cat] = new(widget.Clickable)
					}
					click := a.categoryButtons[cat]
					if click.Clicked(gtx) {
						a.category = cat
						a.list.Position = layout.Position{}
					}
					return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, button(a.theme, click, cat, strings.EqualFold(a.category, cat)).Layout)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.moreCategoriesButton.Clicked(gtx) {
					a.moreCategories = !a.moreCategories
				}
				return button(a.theme, &a.moreCategoriesButton, choose(a.moreCategories, "Fewer", "More categories"), false).Layout(gtx)
			}),
		)
	})
}
