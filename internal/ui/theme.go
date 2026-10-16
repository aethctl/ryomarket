package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type palette struct {
	bg        color.NRGBA
	rail      color.NRGBA
	surface   color.NRGBA
	surface2  color.NRGBA
	raised    color.NRGBA
	line      color.NRGBA
	strong    color.NRGBA
	bright    color.NRGBA
	cream     color.NRGBA
	subtle    color.NRGBA
	dim       color.NRGBA
	faint     color.NRGBA
	bone      color.NRGBA
	inkOnBone color.NRGBA
	accent    color.NRGBA
	accentInk color.NRGBA
	gold      color.NRGBA
	ok        color.NRGBA
	bad       color.NRGBA
}

var colors = loadPalette()

func loadPalette() palette {
	p := palette{
		bg:        hex(0x000000),
		rail:      hex(0x08090b),
		surface:   hex(0x0a0a0a),
		surface2:  hex(0x111216),
		raised:    hex(0x17191f),
		line:      color.NRGBA{R: 205, G: 196, B: 186, A: 34},
		strong:    color.NRGBA{R: 205, G: 196, B: 186, A: 88},
		bright:    hex(0xcdc4ba),
		cream:     hex(0xb0a9a0),
		subtle:    hex(0x958f87),
		dim:       hex(0x7a756e),
		faint:     color.NRGBA{R: 122, G: 117, B: 110, A: 180},
		bone:      hex(0xcdc4ba),
		inkOnBone: hex(0x000000),
		accent:    hex(0xe2342a),
		accentInk: hex(0x000000),
		gold:      hex(0xd9a441),
		ok:        hex(0x7fbf6a),
		bad:       hex(0xe05a5a),
	}

	wallPath, shellPath, themePath := themePaths()
	wall := readObject(wallPath)
	shell := readObject(shellPath)
	theme := readObject(themePath)
	followWallpaper := true
	if value, ok := theme["followWallpaper"].(bool); ok {
		followWallpaper = value
	}
	named := objectValue(shell["themePalette"])

	pick := func(name string, fallback color.NRGBA) color.NRGBA {
		if value, ok := roleString(named, name); ok {
			if parsed, ok := parseHex(value); ok {
				return parsed
			}
		}
		if followWallpaper {
			if value, ok := roleString(wall, name); ok {
				if parsed, ok := parseHex(value); ok {
					return parsed
				}
			}
		}
		return fallback
	}

	p.bg = pick("surface", p.bg)
	p.rail = pick("surfaceContainerLowest", p.rail)
	p.surface = pick("surface", p.surface)
	p.surface2 = pick("surfaceContainerLow", p.surface2)
	p.raised = pick("surfaceContainer", p.raised)
	p.bright = pick("onSurface", p.bright)
	p.cream = p.bright
	p.subtle = pick("onSurfaceVariant", p.subtle)
	p.dim = withAlpha(p.subtle, 188)
	p.faint = withAlpha(p.subtle, 142)
	p.bone = pick("inverseSurface", p.bone)
	p.inkOnBone = pick("inverseOnSurface", p.inkOnBone)
	p.accent = pick("primary", p.accent)
	p.accentInk = pick("onPrimary", p.accentInk)
	p.line = withAlpha(p.bright, 34)
	p.strong = withAlpha(p.bright, 86)
	return p
}

func themePaths() (string, string, string) {
	home, _ := os.UserHomeDir()
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		cacheHome = filepath.Join(home, ".cache")
	}
	return filepath.Join(cacheHome, "ryoku", "colors.json"), filepath.Join(configHome, "ryoku", "shell.json"), filepath.Join(configHome, "ryoku", "theme.json")
}

func themeStateKey() string {
	paths := make([]string, 0, 3)
	wall, shell, theme := themePaths()
	for _, path := range []string{wall, shell, theme} {
		info, err := os.Stat(path)
		if err != nil {
			paths = append(paths, path+":missing")
			continue
		}
		paths = append(paths, fmt.Sprintf("%s:%d:%d", path, info.ModTime().UnixNano(), info.Size()))
	}
	return strings.Join(paths, "|")
}

func readObject(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out map[string]any
	if json.Unmarshal(data, &out) != nil {
		return nil
	}
	return out
}

func uiTypeface() string {
	_, shellPath, _ := themePaths()
	shell := readObject(shellPath)
	if value, ok := shell["fontFamily"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return "Space Grotesk"
}

func roleString(raw map[string]any, key string) (string, bool) {
	if raw == nil {
		return "", false
	}
	keys := []string{key, camelToSnake(key)}
	if value, ok := firstString(raw, keys); ok {
		return value, true
	}
	for _, nested := range []string{"roles", "material", "palette"} {
		if child := objectValue(raw[nested]); child != nil {
			if value, ok := firstString(child, keys); ok {
				return value, true
			}
		}
	}
	if child := objectValue(raw["colors"]); child != nil {
		if value, ok := firstString(child, keys); ok {
			return value, true
		}
		for _, scheme := range []string{"dark", "amoled", "default", "light"} {
			if palette := objectValue(child[scheme]); palette != nil {
				if value, ok := firstString(palette, keys); ok {
					return value, true
				}
			}
		}
	}
	return "", false
}

func objectValue(value any) map[string]any {
	if value == nil {
		return nil
	}
	if object, ok := value.(map[string]any); ok {
		return object
	}
	return nil
}

func firstString(raw map[string]any, keys []string) (string, bool) {
	for _, key := range keys {
		if value, ok := raw[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}

func camelToSnake(value string) string {
	var out strings.Builder
	for i, r := range value {
		if unicode.IsUpper(r) {
			if i > 0 {
				out.WriteByte('_')
			}
			out.WriteRune(unicode.ToLower(r))
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func parseHex(value string) (color.NRGBA, bool) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "#"))
	if len(value) != 6 && len(value) != 8 {
		return color.NRGBA{}, false
	}
	n, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return color.NRGBA{}, false
	}
	if len(value) == 6 {
		return color.NRGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}, true
	}
	return color.NRGBA{R: uint8(n >> 24), G: uint8(n >> 16), B: uint8(n >> 8), A: uint8(n)}, true
}

func newTheme() *material.Theme {
	th := material.NewTheme()
	th.Palette.Bg = colors.bg
	th.Palette.Fg = colors.bright
	th.Palette.ContrastBg = colors.bone
	th.Palette.ContrastFg = selectedInk()
	th.Face = font.Typeface(uiTypeface())
	th.TextSize = unit.Sp(14)
	return th
}

func hex(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

func withAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	c.A = alpha
	return c
}

func selectedInk() color.NRGBA {
	return hex(0x000000)
}

func contrastInk(bg color.NRGBA) color.NRGBA {
	dark := hex(0x0d0d0f)
	light := hex(0xf4f2ed)
	if contrastRatio(dark, bg) >= contrastRatio(light, bg) {
		return dark
	}
	return light
}

func contrastRatio(a, b color.NRGBA) float64 {
	la := relativeLuminance(a)
	lb := relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func relativeLuminance(c color.NRGBA) float64 {
	channel := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

func fill(gtx layout.Context, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: gtx.Constraints.Max}.Op())
}

func roundedBackground(gtx layout.Context, bg color.NRGBA, radius unit.Dp, content layout.Widget) layout.Dimensions {
	return layout.Background{}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Min
		r := gtx.Dp(radius)
		paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: size}, r).Op(gtx.Ops))
		return layout.Dimensions{Size: size}
	}, content)
}

func boxed(gtx layout.Context, bg color.NRGBA, border color.NRGBA, inset unit.Dp, content layout.Widget) layout.Dimensions {
	return widget.Border{Color: border, CornerRadius: unit.Dp(6), Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return roundedBackground(gtx, bg, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(inset).Layout(gtx, content)
		})
	})
}

func label(th *material.Theme, size unit.Sp, text string, c color.NRGBA) material.LabelStyle {
	style := material.Label(th, size, text)
	style.Color = c
	return style
}

func mediumLabel(th *material.Theme, size unit.Sp, text string, c color.NRGBA) material.LabelStyle {
	style := label(th, size, text, c)
	style.Font.Weight = font.Medium
	return style
}

func displayLabel(th *material.Theme, size unit.Sp, text string, c color.NRGBA) material.LabelStyle {
	style := label(th, size, text, c)
	style.Font.Typeface = font.Typeface("Fraunces")
	return style
}

func jpLabel(th *material.Theme, size unit.Sp, text string, c color.NRGBA) material.LabelStyle {
	style := label(th, size, text, c)
	style.Font.Typeface = font.Typeface("Noto Sans CJK JP")
	return style
}

func button(th *material.Theme, click *widget.Clickable, text string, primary bool) material.ButtonStyle {
	style := material.Button(th, click, text)
	style.CornerRadius = unit.Dp(6)
	style.TextSize = unit.Sp(12)
	style.Inset = layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(15), Right: unit.Dp(15)}
	if primary {
		style.Background = colors.bone
		style.Color = hex(0x000000)
		style.Font.Weight = font.SemiBold
	} else {
		style.Background = colors.surface2
		style.Color = colors.cream
	}
	return style
}
