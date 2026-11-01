package ui

import (
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"image"
	"math"
	"time"
)

type hoverState struct {
	from, to float64
	start    time.Time
}

func (a *App) hoverLift(gtx layout.Context, id string, click *widget.Clickable, content layout.Widget) layout.Dimensions {
	if a.reducedMotion.Value {
		return content(gtx)
	}
	if a.hoverStates == nil {
		a.hoverStates = map[string]*hoverState{}
	}
	state := a.hoverStates[id]
	if state == nil {
		state = &hoverState{}
		a.hoverStates[id] = state
	}
	value := state.to
	if !state.start.IsZero() {
		t := gtx.Now.Sub(state.start).Seconds() / 0.18
		if t < 1 {
			value = state.to + (state.from-state.to)*(1+8*t)*math.Exp(-8*t)
		} else {
			state.start = time.Time{}
		}
	}
	target := 0.0
	if click.Hovered() || gtx.Focused(click) {
		target = 1
	}
	if target != state.to {
		state.from = value
		state.to = target
		state.start = gtx.Now
	}
	if !state.start.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second / 60)})
	}
	defer op.Offset(image.Pt(0, -int(2*value))).Push(gtx.Ops).Pop()
	return content(gtx)
}
