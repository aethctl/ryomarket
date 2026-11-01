package ui

import (
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/Ryoku-dev/ryomarket/internal/catalog"
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"github.com/Ryoku-dev/ryomarket/internal/provider"
	"github.com/Ryoku-dev/ryomarket/internal/service"
	"github.com/Ryoku-dev/ryomarket/internal/state"
	"image"
	"testing"
	"time"
)

func storefrontFixture(t *testing.T) *App {
	t.Helper()
	c, err := catalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	return &App{catalog: c, service: service.New(c), state: &state.Store{}, selected: model.DistroNixOS, theme: newTheme(), source: sourceAll, category: "All", view: viewBrowse,
		cardButtons: map[string]*widget.Clickable{}, images: map[string]paint.ImageOp{}, screenshots: map[string]map[int]paint.ImageOp{}}
}

func TestOfficialSearchRemainsVisibleWithoutInstallProvider(t *testing.T) {
	a := storefrontFixture(t)
	a.query.SetText("Ryotunes")
	a.ensureDiscoverCache()
	if len(a.discoverApps) == 0 || a.discoverApps[0].ID != "ryoku:ryotunes" {
		t.Fatal("official name match was not surfaced")
	}
	if len(a.discoverApps[0].Variants) != 0 {
		t.Fatal("search invented an install provider")
	}
}

func TestFeaturedLayoutFitsDesktopAndCompactWidths(t *testing.T) {
	a := storefrontFixture(t)
	for _, width := range []int{620, 860, 1120} {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Now: time.Now(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, 700)}}
		dims := a.collectionFeature(gtx)
		if dims.Size.X > width || dims.Size.Y > 320 {
			t.Fatalf("feature overflow at %d: %v", width, dims.Size)
		}
	}
}

func TestAppearanceModesHaveReadableContrast(t *testing.T) {
	for _, mode := range []string{"dark", "light"} {
		p := applyAppearance(loadPalette(), state.Appearance{Mode: mode})
		if contrastRatio(p.bright, p.bg) < 7 || contrastRatio(p.subtle, p.bg) < 4.5 || contrastRatio(p.accentInk, p.accent) < 4.5 || contrastRatio(p.accent, p.surface2) < 4.5 {
			t.Fatalf("insufficient %s contrast", mode)
		}
	}
	p := applyAppearance(neutralPalette(false), state.Appearance{Surface: "#f6f5f1", Accent: "#73483b"})
	if contrastRatio(p.bright, p.bg) < 7 {
		t.Fatal("surface override did not adapt foregrounds")
	}
}

func TestPreferredProviderFallbackExplanation(t *testing.T) {
	if preferredFallbackText(model.ProviderFlatpak, model.ProviderFlatpak) != "" {
		t.Fatal("unexpected fallback")
	}
	if preferredFallbackText(model.ProviderFlatpak, model.ProviderNixProfile) == "" {
		t.Fatal("fallback must be explained")
	}
}

func TestEmbeddedTypographyAndArtwork(t *testing.T) {
	for _, name := range []string{"SpaceGrotesk-Regular.ttf", "SpaceGrotesk-SemiBold.ttf"} {
		if data, err := typeAssets.ReadFile("assets/" + name); err != nil || len(data) == 0 {
			t.Fatal("missing static font", name)
		}
	}
	a := storefrontFixture(t)
	for _, id := range []string{"ryoku:ryotunes", "ryoku:ryomanager"} {
		a.loadLocalPreview(id)
		if _, ok := a.screenshot(id, 0); !ok {
			t.Fatal("missing authentic preview", id)
		}
	}
}

func TestAppearanceSaveUsesOnlyApplicationState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	st, err := state.Open()
	if err != nil {
		t.Fatal(err)
	}
	old := colors
	defer func() { colors = old }()
	a := &App{state: st, theme: newTheme()}
	a.appearanceMode.Value = "light"
	a.accentEditor.SetText("#73483b")
	a.surfaceEditor.SetText("#f6f5f1")
	a.reducedMotion.Value = true
	a.saveAppearance()
	restored, err := state.Open()
	if err != nil {
		t.Fatal(err)
	}
	if restored.Data.Appearance.Mode != "light" || !restored.Data.Appearance.ReducedMotion || !restored.Data.WelcomeDone {
		t.Fatal("appearance and welcome state not saved")
	}
	if contrastRatio(colors.bright, colors.bg) < 7 {
		t.Fatal("saved surface is not readable")
	}
}

func TestGallerySkipsFailedScreenshotSlots(t *testing.T) {
	a := storefrontFixture(t)
	a.screenshots["example"] = map[int]paint.ImageOp{2: {}, 5: {}}
	indices := a.screenshotIndices("example")
	if len(indices) != 2 || indices[0] != 2 || indices[1] != 5 {
		t.Fatalf("invalid loaded screenshot ordering: %v", indices)
	}
}

func TestUnavailableHostSourceIsHidden(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	a := storefrontFixture(t)
	for _, source := range a.visibleSources() {
		if source.ID == "flatpak" || source.ID == "nix-profile" {
			t.Fatalf("unavailable NixOS source exposed: %s", source.ID)
		}
	}
}

func TestDeclarativePreferenceSelectsGuidanceWithoutAHelper(t *testing.T) {
	a := storefrontFixture(t)
	a.state.Data.PreferredProvider = model.ProviderNixConfig
	item, ok := a.catalog.App("gimp")
	if !ok {
		t.Fatal("missing GIMP")
	}
	selected := a.defaultVariant(item)
	if selected < 0 || item.Variants[model.DistroNixOS][selected].Kind != model.ProviderNixConfig {
		t.Fatal("declarative preference was ignored")
	}
	if a.pending != nil {
		t.Fatal("selecting a preference must not create a transaction")
	}
}

func TestCarouselWrapAndReducedMotion(t *testing.T) {
	a := storefrontFixture(t)
	count := len(a.featuredApplications())
	if count < 3 {
		t.Fatal("featured selection too small")
	}
	a.reducedMotion.Value = true
	a.changeFeature(-1)
	if a.featureIndex != count-1 || !a.featureMotionStart.IsZero() {
		t.Fatal("manual wrap or reduced motion failed")
	}
	a.changeFeature(1)
	if a.featureIndex != 0 {
		t.Fatal("carousel did not wrap forwards")
	}
}

func TestSkipSetupPreservesExistingPreferences(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	st, err := state.Open()
	if err != nil {
		t.Fatal(err)
	}
	st.Data.PreferredProvider = model.ProviderNixProfile
	st.Data.Appearance.Mode = "dark"
	a := &App{state: st, welcome: true}
	a.appearanceMode.Value = "light"
	a.preference.Value = string(model.ProviderFlatpak)
	a.skipOnboarding()
	restored, err := state.Open()
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Data.WelcomeDone || restored.Data.PreferredProvider != model.ProviderNixProfile || restored.Data.Appearance.Mode != "dark" {
		t.Fatal("skip setup changed saved preferences")
	}
}

func TestScreenshotPlacementKeepsEveryEdge(t *testing.T) {
	for _, source := range []image.Point{{1418, 997}, {1432, 996}, {1430, 990}, {997, 1418}} {
		for _, bounds := range []image.Point{{560, 240}, {713, 399}, {273, 221}, {640, 400}} {
			rect, scale := screenshotPlacement(source, bounds)
			if scale <= 0 || rect.Min.X < 0 || rect.Min.Y < 0 || rect.Max.X > bounds.X || rect.Max.Y > bounds.Y {
				t.Fatalf("image outside viewport: %v in %v: %v", source, bounds, rect)
			}
			if float32(rect.Dx()) < float32(source.X)*scale || float32(rect.Dy()) < float32(source.Y)*scale {
				t.Fatalf("source edge clipped: %v in %v: %v at %f", source, bounds, rect, scale)
			}
		}
	}
	a := storefrontFixture(t)
	a.loadLocalPreview("ryoku:ryotunes")
	if got := a.screenshotIndices("ryoku:ryotunes"); len(got) != 4 {
		t.Fatalf("expected all four supplied screenshots, got %v", got)
	}
	for i := 0; i < 4; i++ {
		shot, ok := a.screenshot("ryoku:ryotunes", i)
		if !ok || shot.Size().X < 1400 || shot.Size().Y < 980 {
			t.Fatalf("missing or resized source %d: %v", i, shot.Size())
		}
	}
}

func TestMethodSelectionUpdatesBeforeDetailLayout(t *testing.T) {
	a := storefrontFixture(t)
	a.variantSelection = map[string]int{"test": 0}
	a.providerButtons = map[string]*widget.Clickable{}
	item := model.Application{ID: "test", Variants: map[model.Distro][]model.Variant{model.DistroNixOS: {
		{Kind: model.ProviderNixConfig, Package: "first"}, {Kind: model.ProviderNixConfig, Package: "second"},
	}}}
	a.pending = &provider.Plan{AppID: item.ID}
	a.showConfig = true
	var ops op.Ops
	gtx := layout.Context{Ops: &ops}
	a.methodControl(item, 1).Click()
	a.updateMethodSelection(gtx, item)
	if a.variantSelection[item.ID] != 1 || a.pending != nil || a.showConfig {
		t.Fatal("selection and action state were not updated in the input frame")
	}
	a.busy = true
	a.methodControl(item, 0).Click()
	a.updateMethodSelection(gtx, item)
	if a.variantSelection[item.ID] != 1 {
		t.Fatal("selection changed during a transaction")
	}
}

func TestPreferenceSelectionUpdatesBeforeCardsLayout(t *testing.T) {
	a := storefrontFixture(t)
	a.preference.Value = ""
	a.preferenceButtons = map[string]*widget.Clickable{string(model.ProviderNixConfig): new(widget.Clickable)}
	var ops op.Ops
	gtx := layout.Context{Ops: &ops}
	a.preferenceButtons[string(model.ProviderNixConfig)].Click()
	a.updatePreferenceSelection(gtx, []model.ProviderKind{"", model.ProviderNixConfig})
	if a.preference.Value != string(model.ProviderNixConfig) {
		t.Fatal("preferred method did not change in the input frame")
	}
}
