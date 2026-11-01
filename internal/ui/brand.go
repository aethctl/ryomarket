package ui

import (
	"bytes"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"image"
	"image/color"
)

var ryokuMark = func() paint.ImageOp {
	data, err := officialIcons.ReadFile("assets/ryoku-mark.png")
	if err != nil {
		panic(err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	return paint.NewImageOp(img)
}()

func (a *App) brand(gtx layout.Context, compact bool) layout.Dimensions {
	mark := func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp("Ryoku").Add(gtx.Ops)
		gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(32)), gtx.Dp(unit.Dp(32)))
		gtx.Constraints.Max = gtx.Constraints.Min
		return roundedBackground(gtx, color.NRGBA{R: 12, G: 13, B: 16, A: 255}, unit.Dp(5), func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(2)).Layout(gtx, widget.Image{Src: ryokuMark, Fit: widget.Contain, Position: layout.Center}.Layout)
		})
	}
	if compact {
		return layout.Center.Layout(gtx, mark)
	}
	return layout.Inset{Left: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(mark), layout.Rigid(layout.Spacer{Width: unit.Dp(9)}.Layout), layout.Flexed(1, heading(a.theme, unit.Sp(19), "RyoMarket", colors.bright).Layout),
		)
	})
}
