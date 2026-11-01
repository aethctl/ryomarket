package ui

import (
	"embed"
	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
)

//go:embed assets/SpaceGrotesk-*.ttf assets/SPACE-GROTESK-OFL.txt assets/Fraunces-Regular.ttf assets/FRAUNCES-OFL.txt
var typeAssets embed.FS

var appShaper = func() *text.Shaper {
	var collection []font.FontFace
	for _, name := range []string{"SpaceGrotesk-Regular.ttf", "SpaceGrotesk-SemiBold.ttf", "Fraunces-Regular.ttf"} {
		data, err := typeAssets.ReadFile("assets/" + name)
		if err != nil {
			continue
		}
		faces, err := opentype.ParseCollection(data)
		if err != nil {
			continue
		}
		for i := range faces {
			faces[i].Font.Typeface = "Space Grotesk"
			if name == "Fraunces-Regular.ttf" {
				faces[i].Font.Typeface = "Fraunces"
			}
		}
		collection = append(collection, faces...)
	}
	return text.NewShaper(text.WithCollection(collection))
}()
