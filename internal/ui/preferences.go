package ui

import (
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"github.com/Ryoku-dev/ryomarket/internal/state"
	"image"
	"image/color"
	"strings"
)

func (a *App) preferences(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			sections := []layout.Widget{
				func(gtx layout.Context) layout.Dimensions {
					return a.settingsGroup(gtx, "Appearance", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(a.themeChoices),
							layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
							layout.Rigid(label(a.theme, unit.Sp(12), "Follow Ryoku uses your current desktop colours and font, including Matugen changes.", colors.subtle).Layout),
						)
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.settingsGroup(gtx, "Installation", a.providerPreferences)
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.settingsGroup(gtx, "Accessibility", material.CheckBox(a.theme, &a.reducedMotion, "Reduce animation").Layout)
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.settingsGroup(gtx, "Advanced appearance", a.advancedColours)
				},
			}
			return a.detailList.Layout(gtx, len(sections), func(gtx layout.Context, index int) layout.Dimensions { return sections[index](gtx) })
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if a.saveAppearanceButton.Clicked(gtx) {
				a.saveAppearance()
			}
			if a.resetAppearanceButton.Clicked(gtx) {
				a.appearanceMode.Value = ""
				a.overrides.Value = false
				a.accentEditor.SetText("")
				a.surfaceEditor.SetText("")
			}
			return layout.Inset{Top: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, label(a.theme, unit.Sp(12), choose(a.statusBad, a.status, "Changes apply only to RyoMarket."), chooseColor(a.statusBad, colors.bad, colors.subtle)).Layout),
					layout.Rigid(button(a.theme, &a.resetAppearanceButton, "Reset appearance", false).Layout), layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout), layout.Rigid(button(a.theme, &a.saveAppearanceButton, "Save preferences", true).Layout),
				)
			})
		}),
	)
}

func (a *App) settingsGroup(gtx layout.Context, title string, content layout.Widget) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return roundedBackground(gtx, colors.surface2, unit.Dp(9), func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(heading(a.theme, unit.Sp(18), title, colors.bright).Layout), layout.Rigid(layout.Spacer{Height: unit.Dp(15)}.Layout), layout.Rigid(content),
				)
			})
		})
	})
}

func (a *App) themeChoices(gtx layout.Context) layout.Dimensions {
	if a.themeButtons == nil {
		a.themeButtons = map[string]*widget.Clickable{}
	}
	var children []layout.FlexChild
	for i, choice := range []struct{ id, name string }{{"dark", "Dark"}, {"light", "Light"}, {"", "Follow Ryoku"}} {
		i := i
		choice := choice
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			click := a.themeButtons[choice.id]
			if click == nil {
				click = new(widget.Clickable)
				a.themeButtons[choice.id] = click
			}
			if click.Clicked(gtx) {
				a.appearanceMode.Value = choice.id
			}
			selected := a.appearanceMode.Value == choice.id
			p := applyAppearance(loadPalette(), state.Appearance{Mode: choice.id})
			border := colors.line
			if selected || gtx.Focused(click) {
				border = colors.accent
			}
			return layout.Inset{Right: choose(i < 2, unit.Dp(12), unit.Dp(0))}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					semantic.LabelOp(choice.name + " appearance").Add(gtx.Ops)
					semantic.SelectedOp(selected).Add(gtx.Ops)
					return widget.Border{Color: border, Width: unit.Dp(1), CornerRadius: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return roundedBackground(gtx, colors.surface, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions { return themeSample(gtx, p) }),
									layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
									layout.Rigid(heading(a.theme, unit.Sp(13), choice.name, chooseColor(selected, colors.accent, colors.bright)).Layout),
								)
							})
						})
					})
				})
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func themeSample(gtx layout.Context, p palette) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(72)))
	paint.FillShape(gtx.Ops, p.bg, clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(unit.Dp(5))).Op(gtx.Ops))
	rail := size.X / 5
	paint.FillShape(gtx.Ops, p.rail, clip.Rect(image.Rect(0, 0, rail, size.Y)).Op())
	pad := gtx.Dp(unit.Dp(8))
	x := rail + pad
	paint.FillShape(gtx.Ops, p.surface2, clip.UniformRRect(image.Rect(x, pad, size.X-pad, size.Y-pad), gtx.Dp(unit.Dp(4))).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, p.accent, clip.UniformRRect(image.Rect(x+pad, size.Y/2, size.X-pad*2, size.Y-pad*2), gtx.Dp(unit.Dp(3))).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, p.bright, clip.Rect(image.Rect(x+pad, pad*2, x+pad+max(12, (size.X-x)/3), pad*2+gtx.Dp(unit.Dp(3)))).Op())
	return layout.Dimensions{Size: size}
}

func (a *App) updatePreferenceSelection(gtx layout.Context, kinds []model.ProviderKind) {
	for _, kind := range kinds {
		key := string(kind)
		click := a.preferenceButtons[key]
		if click != nil && click.Clicked(gtx) && a.preferenceAvailable(kind) {
			a.preference.Value = key
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (a *App) providerPreferences(gtx layout.Context) layout.Dimensions {
	if a.preferenceButtons == nil {
		a.preferenceButtons = map[string]*widget.Clickable{}
	}
	kinds := []model.ProviderKind{"", model.ProviderNixFlake, model.ProviderFlatpak, model.ProviderNixConfig, model.ProviderNixProfile, model.ProviderAppImage}
	a.updatePreferenceSelection(gtx, kinds)
	cols := 2
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(label(a.theme, unit.Sp(13), "Preferred method when supported by the application.", colors.subtle).Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var rows []layout.FlexChild
			for row := 0; row < (len(kinds)+cols-1)/cols; row++ {
				row := row
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					var children []layout.FlexChild
					for col := 0; col < cols; col++ {
						col := col
						index := row*cols + col
						children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							if index >= len(kinds) {
								return layout.Dimensions{}
							}
							kind := kinds[index]
							key := string(kind)
							click := a.preferenceButtons[key]
							if click == nil {
								click = new(widget.Clickable)
								a.preferenceButtons[key] = click
							}
							available := a.preferenceAvailable(kind)
							if !available {
								gtx = gtx.Disabled()
							}
							border := colors.line
							selected := a.preference.Value == key
							if selected || gtx.Focused(click) {
								border = colors.accent
							}
							text := methodDescription(kind)
							if !available {
								text = "Not available on this system."
							}
							return layout.Inset{Right: choose(col < cols-1, unit.Dp(10), unit.Dp(0)), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									semantic.LabelOp(methodName(kind) + ". " + text).Add(gtx.Ops)
									semantic.SelectedOp(selected).Add(gtx.Ops)
									semantic.EnabledOp(available).Add(gtx.Ops)
									return widget.Border{Color: border, Width: unit.Dp(1), CornerRadius: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return roundedBackground(gtx, colors.surface, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
											return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
													layout.Rigid(heading(a.theme, unit.Sp(13), methodName(kind), chooseColor(selected, colors.accent, colors.bright)).Layout),
													layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
													layout.Rigid(label(a.theme, unit.Sp(12), text, colors.subtle).Layout),
												)
											})
										})
									})
								})
							})
						}))
					}
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		}),
	)
}

func (a *App) advancedColours(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.CheckBox(a.theme, &a.overrides, "Custom accent and surface colours").Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !a.overrides.Value {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.colourEditor(gtx, &a.accentEditor, "Accent", colors.accent)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(18)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.colourEditor(gtx, &a.surfaceEditor, "Surface", colors.bg)
					}),
				)
			})
		}),
	)
}

func (a *App) colourEditor(gtx layout.Context, editor *widget.Editor, name string, fallbackColor color.NRGBA) layout.Dimensions {
	c := fallbackColor
	if parsed, ok := parseHex(editor.Text()); ok {
		c = parsed
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Dp(unit.Dp(28)), gtx.Dp(unit.Dp(28)))
			paint.FillShape(gtx.Ops, c, clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(unit.Dp(5))).Op(gtx.Ops))
			return layout.Dimensions{Size: size}
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
		layout.Flexed(1, material.Editor(a.theme, editor, name+" hex colour").Layout),
	)
}

func (a *App) saveAppearance() {
	accent, surface := "", ""
	if a.overrides.Value {
		accent = strings.TrimSpace(a.accentEditor.Text())
		surface = strings.TrimSpace(a.surfaceEditor.Text())
		for _, value := range []string{accent, surface} {
			if value != "" {
				if _, ok := parseHex(value); !ok {
					a.status, a.statusBad = "Enter a valid hex colour such as #c6b7a2.", true
					return
				}
			}
		}
	}
	previous := a.state.Data
	a.state.Data.PreferredProvider = model.ProviderKind(a.preference.Value)
	a.state.Data.Appearance = state.Appearance{Mode: a.appearanceMode.Value, Accent: accent, Surface: surface, ReducedMotion: a.reducedMotion.Value}
	a.state.Data.WelcomeDone = true
	if err := a.state.Save(); err != nil {
		a.state.Data = previous
		a.status, a.statusBad = err.Error(), true
		return
	}
	colors = applyAppearance(loadPalette(), a.state.Data.Appearance)
	a.theme = newTheme()
	a.status, a.statusBad = "Preferences saved.", false
	if a.welcome {
		a.welcome = false
		a.navigate(viewDiscover)
	}
}

func (a *App) skipOnboarding() {
	previous := a.state.Data.WelcomeDone
	a.state.Data.WelcomeDone = true
	if err := a.state.Save(); err != nil {
		a.state.Data.WelcomeDone = previous
		a.status, a.statusBad = err.Error(), true
		return
	}
	a.welcome = false
	a.appearanceMode.Value = a.state.Data.Appearance.Mode
	a.preference.Value = string(a.state.Data.PreferredProvider)
	a.navigate(viewDiscover)
}

func (a *App) onboarding(gtx layout.Context) layout.Dimensions {
	return layout.UniformInset(unit.Dp(28)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.skipWelcome.Clicked(gtx) {
					a.skipOnboarding()
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.brand(gtx, false) }), layout.Rigid(button(a.theme, &a.skipWelcome, "Skip setup", false).Layout))
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(850)))
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(label(a.theme, unit.Sp(12), choose(a.welcomeStep == 0, "1 of 2 · Appearance", "2 of 2 · Installation"), colors.subtle).Layout),
						layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
						layout.Rigid(heading(a.theme, unit.Sp(29), choose(a.welcomeStep == 0, "Welcome to RyoMarket", "Choose your preferred method"), colors.bright).Layout),
						layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if a.welcomeStep == 0 {
								return a.themeChoices(gtx)
							}
							return a.providerPreferences(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
						layout.Rigid(label(a.theme, unit.Sp(12), choose(a.welcomeStep == 0, "Follow Ryoku inherits the live desktop theme, including Matugen. You can change this later.", "The selected method is used where supported. Nothing is installed during setup."), colors.subtle).Layout),
						layout.Rigid(layout.Spacer{Height: unit.Dp(22)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if a.welcomeBack.Clicked(gtx) {
								a.welcomeStep = 0
							}
							if a.welcomeContinue.Clicked(gtx) {
								a.advanceOnboarding()
							}
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if a.welcomeStep == 0 {
										return layout.Dimensions{}
									}
									return layout.Inset{Right: unit.Dp(10)}.Layout(gtx, button(a.theme, &a.welcomeBack, "Back", false).Layout)
								}),
								layout.Rigid(button(a.theme, &a.welcomeContinue, choose(a.welcomeStep == 0, "Continue", "Start exploring"), true).Layout),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !a.statusBad {
								return layout.Dimensions{}
							}
							return label(a.theme, unit.Sp(12), a.status, colors.bad).Layout(gtx)
						}),
					)
				})
			}),
		)
	})
}

func (a *App) advanceOnboarding() {
	if a.welcomeStep == 0 {
		a.welcomeStep = 1
	} else {
		a.saveAppearance()
	}
}
