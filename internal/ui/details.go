package ui

import (
	"fmt"
	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"image"
	"math"
	"strings"
)

func (a *App) previewImage(gtx layout.Context, item model.Application, index int) layout.Dimensions {
	a.loadLocalPreview(item.ID)
	if shot, ok := a.screenshot(item.ID, index); ok {
		bounds := gtx.Constraints.Max
		rect, scale := screenshotPlacement(shot.Size(), bounds)
		if scale == 0 {
			return layout.Dimensions{}
		}
		defer clip.Rect(rect).Push(gtx.Ops).Pop()
		defer op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(scale, scale)).Offset(layout.FPt(rect.Min))).Push(gtx.Ops).Pop()
		shot.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		return layout.Dimensions{Size: bounds}
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return a.applicationIcon(gtx, item, 96) })
}

func screenshotPlacement(source, bounds image.Point) (image.Rectangle, float32) {
	if source.X <= 0 || source.Y <= 0 || bounds.X <= 0 || bounds.Y <= 0 {
		return image.Rectangle{}, 0
	}
	available := image.Pt(max(1, bounds.X-2), max(1, bounds.Y-2))
	scale := min(float32(available.X)/float32(source.X), float32(available.Y)/float32(source.Y))
	// Round the clip outwards so fractional scaling preserves the final row and column.
	size := image.Pt(int(math.Ceil(float64(float32(source.X)*scale))), int(math.Ceil(float64(float32(source.Y)*scale))))
	origin := image.Pt((bounds.X-size.X)/2, (bounds.Y-size.Y)/2)
	return image.Rectangle{Min: origin, Max: origin.Add(size)}, scale
}

func (a *App) applicationDetail(gtx layout.Context, item model.Application) layout.Dimensions {
	a.loadMetadata(item)
	display := a.enriched(item)
	variants := item.Variants[a.selected]
	a.updateMethodSelection(gtx, item)
	selected, ok := a.variantSelection[item.ID]
	if !ok || selected < 0 || selected >= len(variants) {
		selected = a.defaultVariant(item)
		a.variantSelection[item.ID] = selected
	}
	return a.detailList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.backButton.Clicked(gtx) {
					a.selectedID = ""
					a.pending = nil
				}
				return layout.Inset{Bottom: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.W.Layout(gtx, button(a.theme, &a.backButton, "← Back", false).Layout)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.applicationIcon(gtx, item, 64) }),
					layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := heading(a.theme, unit.Sp(29), display.Name, colors.bright)
								l.MaxLines = 1
								return l.Layout(gtx)
							}),
							layout.Rigid(label(a.theme, unit.Sp(13), display.Developer, colors.subtle).Layout),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if item.Ecosystem != nil && !item.Supports(a.selected) {
							return label(a.theme, unit.Sp(13), "Upcoming", colors.subtle).Layout(gtx)
						}
						gtx.Constraints.Min = image.Point{}
						gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(185)))
						gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(unit.Dp(70)))
						return a.actionArea(gtx, item, selected)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(15)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := label(a.theme, unit.Sp(14), display.Summary, colors.subtle)
				l.MaxLines = 2
				return l.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(24)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(850)) {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.installationPanel(gtx, item, selected) }),
						layout.Rigid(layout.Spacer{Height: unit.Dp(22)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.detailGallery(gtx, item) }),
						layout.Rigid(layout.Spacer{Height: unit.Dp(22)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.applicationAbout(gtx, display) }),
					)
				}
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(0.66, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.detailGallery(gtx, item) }),
								layout.Rigid(layout.Spacer{Height: unit.Dp(24)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.applicationAbout(gtx, display) }),
							)
						})
					}),
					layout.Flexed(0.34, func(gtx layout.Context) layout.Dimensions { return a.installationPanel(gtx, item, selected) }),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.transactionState(gtx, item) }),
		)
	})
}

func (a *App) detailGallery(gtx layout.Context, item model.Application) layout.Dimensions {
	indices := a.screenshotIndices(item.ID)
	count := len(indices)
	if count == 0 {
		return layout.Dimensions{}
	}
	a.galleryIndex = min(a.galleryIndex, count-1)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			height := unit.Dp(min(400, float32(gtx.Constraints.Max.X)/gtx.Metric.PxPerDp*0.64))
			gtx.Constraints.Min.Y = gtx.Dp(height)
			gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
			return a.previewImage(gtx, item, indices[a.galleryIndex])
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if count < 2 {
				return layout.Dimensions{}
			}
			if a.galleryPrevious.Clicked(gtx) {
				a.galleryIndex = (a.galleryIndex - 1 + count) % count
			}
			if a.galleryNext.Clicked(gtx) {
				a.galleryIndex = (a.galleryIndex + 1) % count
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(button(a.theme, &a.galleryPrevious, "←", false).Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, label(a.theme, unit.Sp(12), fmt.Sprintf("%d of %d", a.galleryIndex+1, count), colors.subtle).Layout)
				}),
				layout.Rigid(button(a.theme, &a.galleryNext, "→", false).Layout),
			)
		}),
	)
}

func (a *App) applicationAbout(gtx layout.Context, item model.Application) layout.Dimensions {
	description := strings.TrimSpace(item.Description)
	if description == "" || description == strings.TrimSpace(item.Summary) {
		return layout.Dimensions{}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(heading(a.theme, unit.Sp(19), "About", colors.bright).Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(label(a.theme, unit.Sp(14), description, colors.cream).Layout),
	)
}

func (a *App) installationPanel(gtx layout.Context, item model.Application, selected int) layout.Dimensions {
	if item.Ecosystem != nil && !item.Supports(a.selected) {
		return roundedBackground(gtx, colors.surface2, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(18)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(heading(a.theme, unit.Sp(18), "Upcoming", colors.bright).Layout), layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout), layout.Rigid(label(a.theme, unit.Sp(13), "Independent installation is not available yet.", colors.subtle).Layout),
				)
			})
		})
	}
	variants := item.Variants[a.selected]
	children := []layout.FlexChild{layout.Rigid(heading(a.theme, unit.Sp(18), "Installation", colors.bright).Layout), layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout)}
	for i := range variants {
		i := i
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.methodOption(gtx, item, i, i == selected) }))
	}
	if len(variants) == 0 {
		children = append(children, layout.Rigid(label(a.theme, unit.Sp(13), "No compatible installation method is available.", colors.subtle).Layout))
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if selected < 0 || selected >= len(variants) {
			return layout.Dimensions{}
		}
		variant := variants[selected]
		if variant.Kind == model.ProviderNixConfig && a.showConfig {
			return a.nixConfigGuide(gtx, variant)
		}
		text := strings.TrimSpace(preferredFallbackText(a.state.Data.PreferredProvider, variant.Kind) + "\n" + variant.Notes)
		if variant.Version != "" {
			text = strings.TrimSpace(text + `
Version ` + variant.Version)
		}
		return label(a.theme, unit.Sp(12), text, colors.subtle).Layout(gtx)
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) transactionState(gtx layout.Context, item model.Application) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if a.pending == nil || a.pending.AppID != item.ID {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return a.planPanel(gtx, *a.pending) })
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if a.transactionApp != item.ID {
				return layout.Dimensions{}
			}
			text := a.status
			if a.busy {
				text = "Working on this application…"
			}
			return layout.Inset{Top: unit.Dp(14)}.Layout(gtx, label(a.theme, unit.Sp(13), text, chooseColor(a.statusBad, colors.bad, colors.subtle)).Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if a.failedPlan == nil || a.failedPlan.AppID != item.ID {
				return layout.Dimensions{}
			}
			if a.retryButton.Clicked(gtx) {
				plan := *a.failedPlan
				a.pending = &plan
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(button(a.theme, &a.retryButton, "Try again", false).Layout),
				layout.Rigid(material.CheckBox(a.theme, &a.transactionLogs, "Technical details").Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !a.transactionLogs.Value {
						return layout.Dimensions{}
					}
					l := label(a.theme, unit.Sp(12), a.transactionOutput, colors.subtle)
					l.MaxLines = 10
					return l.Layout(gtx)
				}),
			)
		}),
	)
}
