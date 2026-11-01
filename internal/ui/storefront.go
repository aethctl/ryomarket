package ui

import (
	"fmt"
	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"image"
	"image/color"
	"math"
	"time"
)

func heading(th *material.Theme, size unit.Sp, value string, c color.NRGBA) material.LabelStyle {
	l := label(th, size, value, c)
	l.Font.Typeface = font.Typeface("Space Grotesk")
	l.Font.Weight = font.SemiBold
	return l
}

func (a *App) cardClick(item model.Application) *widget.Clickable {
	click := a.cardButtons[item.ID]
	if click == nil {
		click = new(widget.Clickable)
		a.cardButtons[item.ID] = click
	}
	return click
}

func (a *App) applicationIcon(gtx layout.Context, item model.Application, size unit.Dp) layout.Dimensions {
	a.loadIcon(item)
	gtx.Constraints.Min = image.Pt(gtx.Dp(size), gtx.Dp(size))
	gtx.Constraints.Max = gtx.Constraints.Min
	if icon, ok := a.artwork(item.ID); ok {
		return widget.Image{Src: icon, Fit: widget.Contain, Position: layout.Center}.Layout(gtx)
	}
	return layout.Center.Layout(gtx, heading(a.theme, unit.Sp(18), initials(item.Name), colors.subtle).Layout)
}

func (a *App) curatedApps(ids []string) []model.Application {
	apps := make([]model.Application, 0, len(ids))
	for _, id := range ids {
		if item, ok := a.catalog.App(id); ok && (item.Ecosystem != nil || item.Supports(a.selected)) {
			apps = append(apps, item)
		}
	}
	return apps
}

func (a *App) featuredApplications() []model.Application {
	return a.curatedApps([]string{"ryoku:ryotunes", "ryoku:ryomanager", "blender", "gimp"})
}

func (a *App) changeFeature(delta int) {
	apps := a.featuredApplications()
	if len(apps) == 0 {
		return
	}
	a.featureFrom = a.featureIndex % len(apps)
	a.featureIndex = (a.featureIndex + delta + len(apps)) % len(apps)
	a.featureMotionStart = time.Now()
	if a.reducedMotion.Value {
		a.featureMotionStart = time.Time{}
	}
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *App) editorialStorefront(gtx layout.Context) layout.Dimensions {
	sections := []layout.Widget{
		a.collectionFeature,
		func(gtx layout.Context) layout.Dimensions {
			return a.discoveryCollection(gtx, "From Ryoku", []string{"ryoku:ryotunes", "ryoku:ryomanager", "ryoku:ryostore", "ryoku:ryomotion", "ryoku:rashin", "ryoku:ryoport"}, false)
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.discoveryCollection(gtx, "Creative tools", []string{"krita", "inkscape", "kdenlive", "obs-studio", "audacity", "io.github.mifi.losslesscut"}, false)
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.discoveryCollection(gtx, "Gaming", []string{"com.valvesoftware.Steam", "net.lutris.Lutris", "com.heroicgameslauncher.hgl", "com.usebottles.bottles", "org.prismlauncher.PrismLauncher", "godot"}, true)
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.discoveryCollection(gtx, "Everyday software", []string{"firefox", "libreoffice", "md.obsidian.Obsidian", "thunderbird", "com.vscodium.codium", "dev.zed.Zed"}, false)
		},
	}
	return a.storefrontList.Layout(gtx, len(sections), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(28)}.Layout(gtx, sections[index])
	})
}

func (a *App) collectionFeature(gtx layout.Context) layout.Dimensions {
	apps := a.featuredApplications()
	if len(apps) == 0 {
		return layout.Dimensions{}
	}
	a.featureIndex %= len(apps)
	if a.featurePrevious.Clicked(gtx) {
		a.changeFeature(-1)
	}
	if a.featureNext.Clicked(gtx) {
		a.changeFeature(1)
	}
	if a.featureOpen.Clicked(gtx) {
		a.openDetail(apps[a.featureIndex])
	}
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(312))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	return roundedBackground(gtx, blendColor(colors.surface2, colors.accent, 0.025), unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				t := 1.0
				if !a.featureMotionStart.IsZero() && !a.reducedMotion.Value {
					t = max(0, gtx.Now.Sub(a.featureMotionStart).Seconds()/0.24)
				}
				if t >= 1 {
					a.featureMotionStart = time.Time{}
					return a.featureSlide(gtx, apps[a.featureIndex])
				}
				gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second / 60)})
				alpha := 1 - math.Pow(1-t, 3)
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						defer paint.PushOpacity(gtx.Ops, float32(1-alpha)).Pop()
						return a.featureSlide(gtx, apps[a.featureFrom%len(apps)])
					}),
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						defer paint.PushOpacity(gtx.Ops, float32(alpha)).Pop()
						defer op.Offset(image.Pt(int(12*(1-alpha)), 0)).Push(gtx.Ops).Pop()
						return a.featureSlide(gtx, apps[a.featureIndex])
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(24), Right: unit.Dp(20), Bottom: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(button(a.theme, &a.featureOpen, "View details", true).Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
						layout.Rigid(button(a.theme, &a.featurePrevious, "←", false).Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if a.featureDots == nil {
								a.featureDots = map[string]*widget.Clickable{}
							}
							var children []layout.FlexChild
							for i, item := range apps {
								i := i
								item := item
								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									click := a.featureDots[item.ID]
									if click == nil {
										click = new(widget.Clickable)
										a.featureDots[item.ID] = click
									}
									if click.Clicked(gtx) {
										a.changeFeature(i - a.featureIndex)
									}
									return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										semantic.LabelOp(fmt.Sprintf("Feature %d of %d: %s", i+1, len(apps), item.Name)).Add(gtx.Ops)
										semantic.SelectedOp(i == a.featureIndex).Add(gtx.Ops)
										size := image.Pt(gtx.Dp(unit.Dp(24)), gtx.Dp(unit.Dp(28)))
										r := gtx.Dp(unit.Dp(3))
										c := colors.dim
										if i == a.featureIndex {
											c = colors.accent
											r = gtx.Dp(unit.Dp(4))
										}
										center := image.Pt(size.X/2, size.Y/2)
										paint.FillShape(gtx.Ops, c, clip.Ellipse(image.Rect(center.X-r, center.Y-r, center.X+r, center.Y+r)).Op(gtx.Ops))
										return layout.Dimensions{Size: size}
									})
								}))
							}
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
						}),
						layout.Rigid(button(a.theme, &a.featureNext, "→", false).Layout),
					)
				})
			}),
		)
	})
}

func (a *App) featureSlide(gtx layout.Context, item model.Application) layout.Dimensions {
	a.loadMetadata(item)
	display := a.enriched(item)
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(0.42, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(24), Left: unit.Dp(26), Right: unit.Dp(14), Bottom: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := unit.Sp(38)
				if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(280)) {
					size = 30
				}
				name := item.Name
				if item.ID == "gimp" {
					name = "GIMP"
				}
				title := label(a.theme, size, name, colors.bright)
				title.Font.Typeface = "Fraunces"
				title.Font.Weight = font.Normal
				title.MaxLines = 1
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(label(a.theme, unit.Sp(12), "Featured", colors.subtle).Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(17)}.Layout),
					layout.Rigid(title.Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := label(a.theme, unit.Sp(14), display.Summary, colors.subtle)
						l.MaxLines = 2
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if item.Ecosystem == nil || item.Supports(a.selected) {
							return layout.Dimensions{}
						}
						return label(a.theme, unit.Sp(12), "Upcoming", colors.subtle).Layout(gtx)
					}),
				)
			})
		}),
		layout.Flexed(0.58, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(16), Right: unit.Dp(18), Bottom: unit.Dp(12), Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				indices := a.screenshotIndices(item.ID)
				index := 0
				if len(indices) > 0 {
					index = indices[0]
				}
				return a.previewImage(gtx, item, index)
			})
		}),
	)
}

func (a *App) discoveryCollection(gtx layout.Context, title string, ids []string, flat bool) layout.Dimensions {
	apps := a.curatedApps(ids)
	if len(apps) == 0 {
		return layout.Dimensions{}
	}
	cols := 3
	if flat {
		cols = 2
	}
	if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(800)) {
		cols = 2
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(heading(a.theme, unit.Sp(21), title, colors.bright).Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var rows []layout.FlexChild
			for row := 0; row < (len(apps)+cols-1)/cols; row++ {
				row := row
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					var children []layout.FlexChild
					for col := 0; col < cols; col++ {
						col := col
						index := row*cols + col
						children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							if index >= len(apps) {
								return layout.Dimensions{}
							}
							return layout.Inset{Right: choose(col < cols-1, unit.Dp(12), unit.Dp(0)), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return a.discoveryTile(gtx, apps[index], flat) })
						}))
					}
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		}),
	)
}

func (a *App) discoveryTile(gtx layout.Context, item model.Application, flat bool) layout.Dimensions {
	click := a.cardClick(item)
	if click.Clicked(gtx) {
		a.openDetail(item)
	}
	bg := colors.surface2
	if flat {
		bg = colors.bg
	}
	if click.Hovered() || gtx.Focused(click) {
		bg = blendColor(bg, colors.accent, 0.07)
	}
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(92))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(item.Name + ", " + item.Summary).Add(gtx.Ops)
		return a.hoverLift(gtx, item.ID, click, func(gtx layout.Context) layout.Dimensions {
			return roundedBackground(gtx, bg, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.applicationIcon(gtx, item, 44) }),
						layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									name := item.Name
									if item.Ecosystem != nil && !item.Supports(a.selected) {
										name += " · Upcoming"
									}
									l := heading(a.theme, unit.Sp(15), name, colors.bright)
									l.MaxLines = 1
									return l.Layout(gtx)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									text := item.Summary
									l := label(a.theme, unit.Sp(12), text, colors.subtle)
									l.MaxLines = 2
									return l.Layout(gtx)
								}),
							)
						}),
					)
				})
			})
		})
	})
}

func (a *App) softwareRow(gtx layout.Context, item model.Application) layout.Dimensions {
	return a.discoveryTile(gtx, item, true)
}

func (a *App) navigate(view string) {
	a.motionFrom = a.view
	a.motionStart = time.Now()
	a.view = view
	a.query.SetText("")
	a.lastQuery = ""
	a.source = sourceAll
	a.category = "All"
	a.selectedID = ""
	a.pending = nil
	a.showConfig = false
	a.list.Position = layout.Position{}
	a.storefrontList.Position = layout.Position{}
	a.detailList.Position = layout.Position{}
	a.sourcesOpen = false
	if view == viewInstalled || view == viewUpdates {
		a.startScan(view == viewUpdates)
	}
}
