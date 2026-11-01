package ui

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/Ryoku-dev/ryomarket/internal/catalog"
	"github.com/Ryoku-dev/ryomarket/internal/metadata"
	"github.com/Ryoku-dev/ryomarket/internal/model"
	"github.com/Ryoku-dev/ryomarket/internal/platform"
	"github.com/Ryoku-dev/ryomarket/internal/provider"
	"github.com/Ryoku-dev/ryomarket/internal/service"
	"github.com/Ryoku-dev/ryomarket/internal/state"
)

const (
	viewDiscover    = "discover"
	viewWelcome     = "welcome"
	viewPreferences = "preferences"
	viewBrowse      = "browse"
	viewInstalled   = "installed"
	viewUpdates     = "updates"
)

type txResult struct {
	plan   provider.Plan
	output string
	err    error
}

type scanResult struct {
	installed []service.Installed
	updates   []service.Update
}

type metadataResult struct {
	appID string
	data  metadata.FlathubApp
	err   error
}

type catalogResult struct {
	source string
	total  int
	err    error
}

type App struct {
	featureIndex       int
	featureFrom        int
	featureMotionStart time.Time
	featurePrevious    widget.Clickable
	featureNext        widget.Clickable
	featureOpen        widget.Clickable
	featureDots        map[string]*widget.Clickable
	themeButtons       map[string]*widget.Clickable
	preferenceButtons  map[string]*widget.Clickable
	overrides          widget.Bool
	advancedAppearance widget.Bool
	skipWelcome        widget.Clickable
	welcomeContinue    widget.Clickable
	welcomeBack        widget.Clickable
	welcomeStep        int
	showConfig         bool
	managementButtons  map[string]*widget.Clickable
	managementRefresh  widget.Clickable
	launchErrors       chan error
	sourceDismiss      widget.Clickable
	sourceSurface      widget.Clickable
	navigationExpanded bool
	navigationList     layout.List
	transactionApp     string
	transactionOutput  string
	failedPlan         *provider.Plan
	retryButton        widget.Clickable
	transactionLogs    widget.Bool
	win                *app.Window
	theme              *material.Theme
	catalog            *catalog.Catalog
	service            *service.Service
	state              *state.Store
	host               platform.Info
	selected           model.Distro

	view                  string
	category              string
	source                string
	selectedID            string
	lastQuery             string
	query                 widget.Editor
	advanced              widget.Bool
	preference            widget.Enum
	preferencesButton     widget.Clickable
	collapseButton        widget.Clickable
	searchButton          widget.Clickable
	filtersButton         widget.Clickable
	moreCategoriesButton  widget.Clickable
	welcomeButton         widget.Clickable
	saveAppearanceButton  widget.Clickable
	resetAppearanceButton widget.Clickable
	sourcesOpen           bool
	moreCategories        bool
	collapsed             bool
	welcome               bool
	reducedMotion         widget.Bool
	appearanceMode        widget.Enum
	accentEditor          widget.Editor
	surfaceEditor         widget.Editor
	galleryPrevious       widget.Clickable
	galleryNext           widget.Clickable
	galleryIndex          int
	motionStart           time.Time
	motionFrom            string
	storefrontList        layout.List
	list                  layout.List
	detailList            layout.List
	categoryList          layout.List

	navButtons        map[string]*widget.Clickable
	categoryButtons   map[string]*widget.Clickable
	sourceButtons     map[string]*widget.Clickable
	cardButtons       map[string]*widget.Clickable
	providerButtons   map[string]*widget.Clickable
	backButton        widget.Clickable
	actionButton      widget.Clickable
	confirmButton     widget.Clickable
	cancelButton      widget.Clickable
	refreshButton     widget.Clickable
	heroPrimaryButton widget.Clickable
	heroDetailsButton widget.Clickable

	variantSelection     map[string]int
	pending              *provider.Plan
	busy                 bool
	checking             map[string]bool
	metadata             map[string]metadata.FlathubApp
	metadataLoading      map[string]bool
	metadataFailedAt     map[string]time.Time
	images               map[string]paint.ImageOp
	screenshots          map[string]map[int]paint.ImageOp
	imageLoading         map[string]bool
	imageFailedAt        map[string]time.Time
	themeKey             string
	installed            []service.Installed
	installedApps        []model.Application
	updates              []service.Update
	updateApps           []model.Application
	scanBusy             bool
	scanUpdatesRequested bool
	status               string
	statusBad            bool
	catalogLoading       bool
	catalogPending       int
	catalogTotal         int
	catalogEpoch         uint64
	discoverCacheKey     string
	discoverApps         []model.Application
	discoverCats         []string
	discoverCount        int
	sourceCounts         map[string]int
	installedIDs         map[string]struct{}

	hoverStates map[string]*hoverState
	txCh        chan txResult
	scanCh      chan scanResult
	metadataCh  chan metadataResult
	imageCh     chan imageResult
	catalogCh   chan catalogResult
	mu          sync.RWMutex
}

func Run() error {
	c, err := catalog.Load()
	if err != nil {
		return err
	}
	if cachedApps, _, ok := catalog.LoadFlathubCache(); ok {
		c.Merge(cachedApps)
	}
	if cachedApps, ok := catalog.LoadAppImageCache(); ok {
		c.Merge(cachedApps)
	}
	nixCacheLoaded := false
	if cachedApps, ok := catalog.LoadNixCache(); ok {
		c.Merge(cachedApps)
		nixCacheLoaded = true
	}
	st, err := state.Open()
	if err != nil {
		return err
	}
	host := platform.Detect()
	if host.Distro == model.DistroNixOS && !nixCacheLoaded {
		if cachedApps, ok := catalog.LoadLegacyNixOSCache(); ok {
			c.Merge(cachedApps)
		}
	}
	for _, distro := range []model.Distro{model.DistroFedora, model.DistroArch, model.DistroCachyOS, model.DistroVoid} {
		if cachedApps, ok := catalog.LoadNativeCache(distro); ok {
			c.Merge(cachedApps)
		}
	}
	selected := host.Distro

	win := new(app.Window)
	win.Option(
		app.Title("RyoMarket"),
		app.Size(unit.Dp(1280), unit.Dp(820)),
		app.MinSize(unit.Dp(760), unit.Dp(600)),
	)

	cachedTotal, providerCounts := c.ProviderCounts()
	catalogPending := 2
	if catalog.NixAvailable() {
		catalogPending++
	}
	if host.Distro != model.DistroUnknown && host.Distro != model.DistroNixOS {
		catalogPending++
	}

	a := &App{
		win:              win,
		theme:            newTheme(),
		catalog:          c,
		service:          service.New(c),
		state:            st,
		host:             host,
		selected:         selected,
		view:             viewDiscover,
		category:         "All",
		source:           sourceAll,
		list:             layout.List{Axis: layout.Vertical},
		detailList:       layout.List{Axis: layout.Vertical},
		categoryList:     layout.List{Axis: layout.Horizontal},
		navButtons:       map[string]*widget.Clickable{},
		categoryButtons:  map[string]*widget.Clickable{},
		sourceButtons:    map[string]*widget.Clickable{},
		cardButtons:      map[string]*widget.Clickable{},
		providerButtons:  map[string]*widget.Clickable{},
		variantSelection: map[string]int{},
		checking:         map[string]bool{},
		metadata:         map[string]metadata.FlathubApp{},
		metadataLoading:  map[string]bool{},
		metadataFailedAt: map[string]time.Time{},
		images:           map[string]paint.ImageOp{},
		screenshots:      map[string]map[int]paint.ImageOp{},
		imageLoading:     map[string]bool{},
		imageFailedAt:    map[string]time.Time{},
		catalogLoading:   catalogPending > 0,
		catalogPending:   catalogPending,
		catalogTotal:     cachedTotal,
		sourceCounts:     buildSourceCounts(cachedTotal, providerCounts),
		installedIDs:     map[string]struct{}{},
		txCh:             make(chan txResult, 4),
		scanCh:           make(chan scanResult, 2),
		metadataCh:       make(chan metadataResult, 4),
		imageCh:          make(chan imageResult, 4),
		catalogCh:        make(chan catalogResult, 4),
	}
	a.preference.Value = string(st.Data.PreferredProvider)
	a.appearanceMode.Value = st.Data.Appearance.Mode
	a.accentEditor.SingleLine, a.surfaceEditor.SingleLine = true, true
	a.accentEditor.SetText(st.Data.Appearance.Accent)
	a.surfaceEditor.SetText(st.Data.Appearance.Surface)
	a.reducedMotion.Value = st.Data.Appearance.ReducedMotion
	a.welcome = !st.Data.WelcomeDone && len(st.Data.Transactions) == 0
	if a.welcome {
		a.view = viewWelcome
	}
	a.featureDots = map[string]*widget.Clickable{}
	a.themeButtons = map[string]*widget.Clickable{}
	a.preferenceButtons = map[string]*widget.Clickable{}
	a.managementButtons = map[string]*widget.Clickable{}
	a.launchErrors = make(chan error, 4)
	a.overrides.Value = st.Data.Appearance.Accent != "" || st.Data.Appearance.Surface != ""
	a.storefrontList.Axis = layout.Vertical
	a.navigationList.Axis = layout.Vertical
	colors = applyAppearance(loadPalette(), st.Data.Appearance)
	a.theme = newTheme()
	a.refreshCatalogStats()
	a.themeKey = themeStateKey()
	a.query.SingleLine = true
	for _, key := range []string{viewDiscover, viewBrowse, viewInstalled, viewUpdates} {
		a.navButtons[key] = new(widget.Clickable)
	}
	if selected != model.DistroUnknown {
		a.startScan(false)
	}
	a.startCatalogRefresh()
	go a.watchTheme()

	go func() {
		if err := a.loop(); err != nil {
			log.Printf("Ryomarket: %v", err)
		}
		os.Exit(0)
	}()
	app.Main()
	return nil
}

func (a *App) loop() error {
	var ops op.Ops
	for {
		switch e := a.win.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			a.consumeAsync()
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) consumeAsync() {
	for {
		select {
		case err := <-a.launchErrors:
			a.status, a.statusBad = err.Error(), true
		default:
			goto metadata
		}
	}
metadata:
	for {
		select {
		case result := <-a.metadataCh:
			a.mu.Lock()
			delete(a.metadataLoading, result.appID)
			if result.err == nil {
				a.metadata[result.appID] = result.data
				delete(a.metadataFailedAt, result.appID)
			} else {
				a.metadataFailedAt[result.appID] = time.Now()
			}
			a.mu.Unlock()
			if result.err == nil {
				a.loadMediaFromMetadata(result.appID, result.data)
			}
		default:
			goto images
		}
	}
images:
	for {
		select {
		case result := <-a.imageCh:
			key := imageAssetKey(result.appID, result.asset, result.index)
			a.mu.Lock()
			delete(a.imageLoading, key)
			if result.ok {
				if result.asset == "icon" {
					a.images[result.appID] = result.op
				} else {
					if a.screenshots[result.appID] == nil {
						a.screenshots[result.appID] = map[int]paint.ImageOp{}
					}
					a.screenshots[result.appID][result.index] = result.op
				}
				delete(a.imageFailedAt, key)
			} else {
				a.imageFailedAt[key] = time.Now()
			}
			a.mu.Unlock()
		default:
			goto transactions
		}
	}
transactions:
	for {
		select {
		case result := <-a.txCh:
			a.busy = false
			if result.err != nil {
				a.transactionOutput = transactionError(result.err, result.output)
				a.failedPlan = &result.plan
				a.status = "The change could not be completed. Try again or choose another source."
				a.statusBad = true
			} else {
				a.status = result.plan.Summary() + " complete"
				a.statusBad = false
			}
			_ = a.state.Record(state.Transaction{
				AppID: result.plan.AppID, AppName: result.plan.AppName, Provider: result.plan.Variant.Kind,
				Package: result.plan.Variant.Package, Operation: result.plan.Operation, Distro: result.plan.Distro,
				At: time.Now(), Success: result.err == nil, Error: errorText(result.err),
			})
			if appItem, ok := a.catalog.App(result.plan.AppID); ok {
				a.inspect(appItem)
			}
			a.startScan(a.view == viewUpdates)
		default:
			goto scans
		}
	}
scans:
	for {
		select {
		case result := <-a.scanCh:
			a.installed = result.installed
			a.installedApps = uniqueApplicationsFromInstalled(result.installed)
			a.updates = result.updates
			a.updateApps = uniqueApplicationsFromUpdates(result.updates)
			a.installedIDs = make(map[string]struct{}, len(result.installed))
			for _, installed := range result.installed {
				a.installedIDs[installed.App.ID] = struct{}{}
			}
			a.scanBusy = false
			if a.scanUpdatesRequested {
				a.scanUpdatesRequested = false
				a.startScan(true)
			}
		default:
			goto catalogue
		}
	}
catalogue:
	for {
		select {
		case result := <-a.catalogCh:
			if a.catalogPending > 0 {
				a.catalogPending--
			}
			a.catalogLoading = a.catalogPending > 0
			if result.err != nil {
				a.status = result.source + " catalogue unavailable: " + result.err.Error()
				a.statusBad = true
				continue
			}
			a.catalogEpoch++
			a.invalidateDiscoverCache()
			a.refreshCatalogStats()
			if strings.Contains(result.source, "repositories") || result.source == "Nixpkgs" {
				a.status = fmt.Sprintf("%s synced · %d packages", result.source, result.total)
				a.statusBad = false
			}
			if a.host.Distro != model.DistroUnknown && !a.scanBusy {
				a.startScan(a.view == viewUpdates)
			}
		default:
			return
		}
	}
}

func (a *App) refreshTheme(gtx layout.Context) {
	key := themeStateKey()
	if key != a.themeKey {
		colors = applyAppearance(loadPalette(), a.state.Data.Appearance)
		a.theme = newTheme()
		a.themeKey = key
	}
}

func (a *App) layout(gtx layout.Context) layout.Dimensions {
	a.refreshTheme(gtx)
	a.shortcuts(gtx)
	fill(gtx, colors.bg)
	if a.view == viewWelcome {
		return a.onboarding(gtx)
	}
	compact := a.collapsed || (!a.navigationExpanded && gtx.Constraints.Max.X < gtx.Dp(unit.Dp(1050)))
	width := unit.Dp(204)
	if compact {
		width = 68
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(width)
			gtx.Constraints.Max.X = gtx.Constraints.Min.X
			return a.navigation(gtx, compact)
		}),
		layout.Flexed(1, a.content),
	)
}

func (a *App) searchBox(gtx layout.Context) layout.Dimensions {
	hint := "Search applications"
	border := colors.line
	if gtx.Focused(&a.query) {
		border = colors.accent
	}
	return boxed(gtx, colors.surface, border, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
		ed := material.Editor(a.theme, &a.query, hint)
		ed.TextSize = unit.Sp(13)
		ed.Color = colors.bright
		ed.HintColor = colors.dim
		ed.SelectionColor = withAlpha(colors.accent, 90)
		return ed.Layout(gtx)
	})
}

func (a *App) content(gtx layout.Context) layout.Dimensions {
	if a.query.Text() != a.lastQuery && strings.TrimSpace(a.query.Text()) != "" && a.view != viewBrowse {
		a.view = viewBrowse
		a.selectedID = ""
		a.list.Position = layout.Position{}
	}
	inset := unit.Dp(32)
	if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(800)) {
		inset = 22
	}
	return layout.Inset{Top: unit.Dp(26), Left: inset, Right: inset, Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.header),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.navigationMotion(gtx, a.body) }),
			layout.Rigid(a.footer),
		)
	})
}

func (a *App) header(gtx layout.Context) layout.Dimensions {
	if a.selectedID != "" {
		return layout.Dimensions{}
	}
	title, sub := "Discover", ""
	switch a.view {
	case viewBrowse:
		title, sub = "Browse all", fmt.Sprintf("%s %s", formatCount(a.headerCount()), choose(a.headerCount() == 1, "application", "applications"))
	case viewInstalled:
		title, sub = "Installed", fmt.Sprintf("%d applications", len(a.installedApps))
	case viewUpdates:
		title, sub = "Updates", "Keep your applications current"
	case viewPreferences:
		title, sub = "Preferences", ""
	}
	if a.selectedID != "" {
		title, sub = "Application", "Software details"
	}

	return layout.Inset{Bottom: unit.Dp(22)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(heading(a.theme, unit.Sp(20), title, colors.bright).Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if sub == "" {
							return layout.Dimensions{}
						}
						return label(a.theme, unit.Sp(12), sub, colors.subtle).Layout(gtx)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.view != viewBrowse {
					return layout.Dimensions{}
				}
				if a.filtersButton.Clicked(gtx) {
					a.sourcesOpen = !a.sourcesOpen
				}
				return button(a.theme, &a.filtersButton, "Sources & categories", false).Layout(gtx)
			}),
		)
	})
}

func (a *App) ensureDiscoverCache() {
	key := fmt.Sprintf("%d\x00%s\x00%s\x00%s", a.catalogEpoch, a.source, a.category, a.query.Text()+fmt.Sprint(a.advanced.Value)+a.view)
	if key == a.discoverCacheKey {
		return
	}
	providers := sourceKinds(a.source)
	if a.source == sourceAll {
		providers = a.browseProviders()
	}
	curated := a.view != viewBrowse && strings.TrimSpace(a.query.Text()) == "" && a.source == sourceAll && a.category == "All"
	result := a.catalog.BrowseFor(a.selected, a.query.Text(), a.category, providers, curated)
	a.discoverCacheKey = key
	a.discoverApps = result.Apps
	if q := strings.ToLower(strings.TrimSpace(a.query.Text())); q != "" && a.source == sourceAll && a.category == "All" {
		var official []model.Application
		for _, app := range catalog.RyokuApplications() {
			if strings.Contains(strings.ToLower(app.Name), q) {
				official = append(official, app)
			}
		}
		a.discoverApps = append(official, a.discoverApps...)
		result.Count += len(official)
	}
	a.discoverCats = result.Categories
	a.discoverCount = result.Count
}

func (a *App) invalidateDiscoverCache() {
	a.discoverCacheKey = ""
}

func (a *App) headerCount() int {
	switch a.view {
	case viewInstalled:
		return len(a.installedApps)
	case viewUpdates:
		return len(a.updateApps)
	default:
		a.ensureDiscoverCache()
		return a.discoverCount
	}
}

func (a *App) body(gtx layout.Context) layout.Dimensions {
	if a.selectedID != "" {
		if item, ok := a.catalog.App(a.selectedID); ok {
			return a.detail(gtx, item)
		}
		a.selectedID = ""
	}
	switch a.view {
	case viewPreferences:
		return a.preferences(gtx)
	case viewInstalled:
		return a.installedView(gtx)
	case viewUpdates:
		return a.updatesView(gtx)
	case viewBrowse:
		return a.browse(gtx)
	default:
		return a.editorialStorefront(gtx)
	}
}

func (a *App) appGrid(gtx layout.Context, apps []model.Application) layout.Dimensions {
	columns := 3
	if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(760)) {
		columns = 2
	}
	if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(520)) {
		columns = 1
	}
	rows := (len(apps) + columns - 1) / columns
	return a.list.Layout(gtx, rows, func(gtx layout.Context, row int) layout.Dimensions {
		gtx.Constraints.Min.Y = 0
		gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(250))
		children := make([]layout.FlexChild, 0, columns)
		for col := 0; col < columns; col++ {
			idx := row*columns + col
			if idx >= len(apps) {
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }))
				continue
			}
			item := apps[idx]
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				right := unit.Dp(0)
				if col < columns-1 {
					right = unit.Dp(10)
				}
				return layout.Inset{Right: right, Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.appCard(gtx, item)
				})
			}))
		}
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
	})
}

func (a *App) appCard(gtx layout.Context, item model.Application) layout.Dimensions {
	a.loadIcon(item)
	click := a.cardClick(item)
	if click.Clicked(gtx) {
		a.openDetail(item)
	}
	height := unit.Dp(174)
	gtx.Constraints.Min.Y = gtx.Dp(height)
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	bg := colors.surface2
	focused := gtx.Focused(click)
	if click.Hovered() || focused {
		bg = blendColor(bg, colors.accent, 0.08)
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return roundedBackground(gtx, bg, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.applicationIcon(gtx, item, 72) }),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								text := ""
								if a.appInstalled(item.ID) {
									text = "Installed"
								}
								if item.Ecosystem != nil && !item.Supports(a.selected) {
									text = "Upcoming"
								}
								return layout.E.Layout(gtx, label(a.theme, unit.Sp(11), text, colors.subtle).Layout)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := heading(a.theme, unit.Sp(16), item.Name, colors.bright)
						l.MaxLines = 1
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						text := a.providerName(item)
						if item.Ecosystem != nil && !item.Supports(a.selected) {
							text = "By Ryoku"
						}
						if click.Hovered() || focused {
							text = item.Summary
						}
						l := label(a.theme, unit.Sp(12), text, colors.subtle)
						l.MaxLines = 1
						return l.Layout(gtx)
					}),
				)
			})
		})
	})
}

func initials(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "?"
	}
	runes := []rune(parts[0])
	if len(parts) == 1 {
		if len(runes) >= 2 {
			return strings.ToUpper(string(runes[:2]))
		}
		return strings.ToUpper(string(runes))
	}
	second := []rune(parts[1])
	if len(runes) == 0 || len(second) == 0 {
		return strings.ToUpper(string([]rune(name)[0]))
	}
	return strings.ToUpper(string([]rune{runes[0], second[0]}))
}

func variantName(variant model.Variant) string {
	if strings.TrimSpace(variant.Label) != "" {
		return variant.Label
	}
	return variant.Kind.DisplayName()
}

func (a *App) providerName(item model.Application) string {
	variants := item.Variants[a.selected]
	idx := a.defaultVariant(item)
	if idx < 0 || idx >= len(variants) {
		return "Unavailable"
	}
	if !a.service.VariantAvailable(a.selected, variants[idx]) {
		return "Unavailable"
	}
	return variantName(variants[idx])
}

func (a *App) appInstalled(appID string) bool {
	_, ok := a.installedIDs[appID]
	return ok
}

func (a *App) openDetail(item model.Application) {
	a.selectedID = item.ID
	a.galleryIndex = 0
	a.variantSelection[item.ID] = a.preferredVariant(item)
	a.pending = nil
	a.detailList.Position = layout.Position{}
	a.inspect(item)
	a.loadMetadata(item)
}

func (a *App) footer(gtx layout.Context) layout.Dimensions {
	status := ""
	statusColor := colors.dim
	if a.catalogLoading {
		status = "Syncing catalogue"
		statusColor = colors.subtle
	} else if a.scanBusy {
		status = "Checking installed applications"
		statusColor = colors.subtle
	} else if a.busy {
		status = "Working"
		statusColor = colors.subtle
	} else if a.statusBad && a.status != "" {
		status = a.status
		statusColor = colors.bad
	}
	if status == "" {
		return layout.Dimensions{}
	}
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := label(a.theme, unit.Sp(11), status, statusColor)
		l.MaxLines = 1
		return l.Layout(gtx)
	})
}

func (a *App) detail(gtx layout.Context, item model.Application) layout.Dimensions {
	return a.applicationDetail(gtx, item)
}

func (a *App) actionArea(gtx layout.Context, item model.Application, selected int) layout.Dimensions {
	if selected < 0 || selected >= len(item.Variants[a.selected]) {
		if item.Ecosystem != nil && !item.Supports(a.selected) {
			return a.empty(gtx, strings.ToUpper(item.Ecosystem.Status), item.Ecosystem.Reason)
		}
		return a.empty(gtx, "NO INSTALL SOURCE", "No compatible installation source is available on this machine.")
	}
	if a.busy {
		return a.compactActionButton(gtx, "WORKING…", false)
	}
	variant := item.Variants[a.selected][selected]
	if variant.Kind == model.ProviderNixConfig && a.host.Distro == model.DistroNixOS {
		if a.actionButton.Clicked(gtx) {
			a.showConfig = !a.showConfig
		}
		return button(a.theme, &a.actionButton, "View configuration", true).Layout(gtx)
	}
	if a.isChecking(item.ID) {
		return a.compactActionButton(gtx, "CHECKING INSTALLATION…", false)
	}

	if !a.service.VariantAvailable(a.selected, variant) {
		title := "NOT AVAILABLE ON THIS HOST"
		message := "Choose another installation source for this system."
		return boxed(gtx, colors.surface2, colors.line, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(mediumLabel(a.theme, unit.Sp(9), title, colors.subtle).Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
				layout.Rigid(label(a.theme, unit.Sp(11), message, colors.dim).Layout),
			)
		})
	}

	isInstalled := a.service.VariantInstalled(item.ID, selected)
	op := provider.Install
	labelText := "INSTALL"
	variantIndex := selected
	if isInstalled {
		if a.view == viewUpdates && a.hasUpdate(item.ID, selected) {
			op = provider.Update
			labelText = "UPDATE"
		} else {
			op = provider.Remove
			labelText = "REMOVE"
		}
	}
	if a.actionButton.Clicked(gtx) {
		plan, err := a.service.Plan(item, a.selected, variantIndex, op)
		if err != nil {
			a.status, a.statusBad = err.Error(), true
		} else {
			a.pending = &plan
		}
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = 0
			style := button(a.theme, &a.actionButton, labelText, op != provider.Remove)
			style.Inset = layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(24), Right: unit.Dp(24)}
			return style.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
	)
}

func (a *App) nixConfigGuide(gtx layout.Context, variant model.Variant) layout.Dimensions {
	pkg := strings.TrimSpace(variant.Package)
	if pkg == "" {
		pkg = "package-name"
	}
	snippet := "environment.systemPackages = with pkgs; [\n  " + pkg + "\n];"
	if variant.FlakeRef != "" {
		snippet = "environment.systemPackages = [\n  (builtins.getFlake " + strconv.Quote(variant.FlakeRef) + ").packages.${pkgs.stdenv.hostPlatform.system}." + pkg + "\n];"
	}
	return boxed(gtx, colors.surface2, colors.line, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(mediumLabel(a.theme, unit.Sp(10), "ADD TO YOUR NIXOS CONFIG", colors.bright).Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(label(a.theme, unit.Sp(10), "Put this in whichever module owns your system packages:", colors.subtle).Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(9)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return boxed(gtx, colors.bg, colors.line, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
					l := label(a.theme, unit.Sp(11), snippet, colors.bright)
					l.Font.Typeface = font.Typeface("SpaceMono Nerd Font, monospace")
					return l.Layout(gtx)
				})
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(label(a.theme, unit.Sp(10), "Then rebuild with your usual nixos-rebuild or flake command.", colors.dim).Layout),
		)
	})
}

func (a *App) hasUpdate(appID string, variantIndex int) bool {
	for _, item := range a.updates {
		if item.App.ID == appID && item.VariantIndex == variantIndex {
			return true
		}
	}
	return false
}

func (a *App) preferredVariant(item model.Application) int {
	if a.view == viewUpdates {
		for _, update := range a.updates {
			if update.App.ID == item.ID {
				return update.VariantIndex
			}
		}
	}
	if a.view == viewInstalled {
		for _, installed := range a.installed {
			if installed.App.ID == item.ID {
				return installed.VariantIndex
			}
		}
	}
	if selected, ok := a.variantSelection[item.ID]; ok {
		return selected
	}
	return a.defaultVariant(item)
}

func (a *App) defaultVariant(item model.Application) int {
	variants := item.Variants[a.selected]
	if kinds := sourceKinds(a.source); len(kinds) == 1 {
		for i, variant := range variants {
			if variant.Kind == kinds[0] {
				return i
			}
		}
	}
	if a.state != nil && a.state.Data.PreferredProvider != "" {
		for i, variant := range variants {
			if variant.Kind == a.state.Data.PreferredProvider && a.methodAvailable(variant) {
				return i
			}
		}
	}
	recommended := item.RecommendedVariant(a.selected)
	if recommended >= 0 && recommended < len(variants) && a.methodAvailable(variants[recommended]) {
		return recommended
	}
	for i, variant := range variants {
		if a.methodAvailable(variant) {
			return i
		}
	}
	return recommended
}

func (a *App) planPanel(gtx layout.Context, plan provider.Plan) layout.Dimensions {
	return boxed(gtx, colors.surface, colors.strong, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(label(a.theme, unit.Sp(10), "CONFIRM TRANSACTION", colors.bright).Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(label(a.theme, unit.Sp(17), plan.Summary(), colors.bright).Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(label(a.theme, unit.Sp(13), preferredFallbackText(a.state.Data.PreferredProvider, plan.Variant.Kind), colors.subtle).Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		}
		if a.advanced.Value {
			for _, step := range plan.Steps {
				step := step
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					prefix := "• "
					if step.Elevated {
						prefix = "◆ "
					}
					return layout.Inset{Bottom: unit.Dp(5)}.Layout(gtx, label(a.theme, unit.Sp(11), prefix+step.Title, colors.subtle).Layout)
				}))
			}
		}
		for _, warning := range plan.Warnings {
			warning := warning
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, label(a.theme, unit.Sp(10), "!  "+warning, colors.gold).Layout)
			}))
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.confirmButton.Clicked(gtx) && !a.busy {
					a.execute(plan)
					a.pending = nil
				}
				if a.cancelButton.Clicked(gtx) {
					a.pending = nil
				}
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(button(a.theme, &a.confirmButton, "CONFIRM", true).Layout),
					layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
					layout.Rigid(button(a.theme, &a.cancelButton, "CANCEL", false).Layout),
				)
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (a *App) installedView(gtx layout.Context) layout.Dimensions { return a.management(gtx, false) }

func (a *App) updatesView(gtx layout.Context) layout.Dimensions { return a.management(gtx, true) }

func (a *App) empty(gtx layout.Context, title, body string) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(520)))
		return boxed(gtx, colors.surface2, colors.line, unit.Dp(24), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(label(a.theme, unit.Sp(18), title, colors.bright).Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(label(a.theme, unit.Sp(12), body, colors.dim).Layout),
			)
		})
	})
}

func (a *App) statusBar(gtx layout.Context) layout.Dimensions {
	if a.status == "" {
		return layout.Dimensions{}
	}
	c := colors.ok
	if a.statusBad {
		c = colors.bad
	}
	return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return boxed(gtx, colors.surface2, c, unit.Dp(9), label(a.theme, unit.Sp(10), a.status, c).Layout)
	})
}

func (a *App) isChecking(appID string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.checking[appID]
}

func (a *App) inspect(item model.Application) {
	if a.isChecking(item.ID) || a.host.Distro == model.DistroUnknown {
		return
	}
	a.mu.Lock()
	a.checking[item.ID] = true
	a.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		a.service.DetectInstalled(ctx, item, a.host.Distro)
		a.mu.Lock()
		delete(a.checking, item.ID)
		a.mu.Unlock()
		a.win.Invalidate()
	}()
}

func (a *App) execute(plan provider.Plan) {
	if a.busy {
		return
	}
	a.busy = true
	a.transactionApp = plan.AppID
	a.failedPlan = nil
	a.transactionOutput = ""
	a.status = plan.Summary()
	a.statusBad = false
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		output, err := a.service.Execute(ctx, plan)
		a.txCh <- txResult{plan: plan, output: output, err: err}
		a.win.Invalidate()
	}()
}

func (a *App) mergeCatalog(apps []model.Application) {
	const chunkSize = 512
	for start := 0; start < len(apps); start += chunkSize {
		end := start + chunkSize
		if end > len(apps) {
			end = len(apps)
		}
		a.catalog.Merge(apps[start:end])
	}
}

func (a *App) startCatalogRefresh() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		items, total, err := (metadata.FlathubClient{}).SearchAll(ctx)
		if err != nil {
			a.catalogCh <- catalogResult{source: "Flathub", err: err}
			a.win.Invalidate()
			return
		}
		apps := catalog.FlathubApplications(items)
		_ = catalog.SaveFlathubCache(apps, total)
		a.mergeCatalog(apps)
		a.catalogCh <- catalogResult{source: "Flathub", total: total}
		a.win.Invalidate()
	}()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
		defer cancel()
		items, err := (metadata.AppImageClient{}).All(ctx)
		if err != nil {
			a.catalogCh <- catalogResult{source: "AppImage", err: err}
			a.win.Invalidate()
			return
		}
		apps := catalog.AppImageApplications(items)
		_ = catalog.SaveAppImageCache(apps)
		a.mergeCatalog(apps)
		a.catalogCh <- catalogResult{source: "AppImage", total: len(apps)}
		a.win.Invalidate()
	}()

	if catalog.NixAvailable() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			apps, err := catalog.NixApplications(ctx)
			if err != nil {
				a.catalogCh <- catalogResult{source: "Nixpkgs", err: err}
				a.win.Invalidate()
				return
			}
			a.mergeCatalog(apps)
			a.catalogCh <- catalogResult{source: "Nixpkgs", total: len(apps)}
			a.win.Invalidate()
		}()
	}

	if a.host.Distro != model.DistroUnknown && a.host.Distro != model.DistroNixOS {
		go func(distro model.Distro) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			apps, err := catalog.NativeApplications(ctx, distro)
			if err != nil {
				a.catalogCh <- catalogResult{source: distro.DisplayName() + " repositories", err: err}
				a.win.Invalidate()
				return
			}
			_ = catalog.SaveNativeCache(distro, apps)
			a.mergeCatalog(apps)
			a.catalogCh <- catalogResult{source: distro.DisplayName() + " repositories", total: len(apps)}
			a.win.Invalidate()
		}(a.host.Distro)
	}
}

func (a *App) startScan(checkUpdates bool) {
	if a.host.Distro == model.DistroUnknown {
		return
	}
	if a.scanBusy {
		a.scanUpdatesRequested = a.scanUpdatesRequested || checkUpdates
		return
	}
	a.scanBusy = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		installed := a.service.ScanInstalled(ctx, a.host.Distro)
		var updates []service.Update
		if checkUpdates && ctx.Err() == nil {
			updates = a.service.ScanUpdates(ctx, a.host.Distro, installed)
		}
		a.scanCh <- scanResult{installed: installed, updates: updates}
		a.win.Invalidate()
	}()
}

func (a *App) loadMetadata(item model.Application) {
	a.loadIcon(item)
	a.loadApplicationScreenshots(item)
	packageID := flatpakID(item)
	if packageID == "" {
		return
	}
	a.mu.Lock()
	_, cached := a.metadata[item.ID]
	loading := a.metadataLoading[item.ID]
	failedAt := a.metadataFailedAt[item.ID]
	cooldown := !failedAt.IsZero() && time.Since(failedAt) < 45*time.Second
	if !cached && !loading && !cooldown {
		a.metadataLoading[item.ID] = true
	}
	a.mu.Unlock()
	if cached || loading || cooldown {
		return
	}
	go func(appID, flatpakID string) {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		select {
		case metadataSlots <- struct{}{}:
			defer func() { <-metadataSlots }()
		case <-ctx.Done():
			a.metadataCh <- metadataResult{appID: appID, err: ctx.Err()}
			return
		}
		data, cached := metadata.LoadAppCache(flatpakID)
		var err error
		if !cached {
			data, err = (metadata.FlathubClient{}).App(ctx, flatpakID)
			if err == nil {
				_ = metadata.SaveAppCache(flatpakID, data)
			}
		}
		a.metadataCh <- metadataResult{appID: appID, data: data, err: err}
		a.win.Invalidate()
	}(item.ID, packageID)
}

func (a *App) metadataFor(appID string) (metadata.FlathubApp, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	data, ok := a.metadata[appID]
	return data, ok
}

func (a *App) enriched(item model.Application) model.Application {
	data, ok := a.metadataFor(item.ID)
	if !ok {
		return item
	}
	if data.Name != "" {
		item.Name = data.Name
	}
	if data.Summary != "" {
		item.Summary = data.Summary
	}
	if data.Description != "" {
		item.Description = data.Description
	}
	if data.DeveloperName != "" {
		item.Developer = data.DeveloperName
	}
	if data.Icon != "" {
		item.Icon = data.Icon
	}
	return item
}

func uniqueApplicationsFromInstalled(items []service.Installed) []model.Application {
	seen := make(map[string]bool, len(items))
	out := make([]model.Application, 0, len(items))
	for _, item := range items {
		if seen[item.App.ID] {
			continue
		}
		seen[item.App.ID] = true
		out = append(out, item.App)
	}
	return out
}

func uniqueApplicationsFromUpdates(items []service.Update) []model.Application {
	seen := make(map[string]bool, len(items))
	out := make([]model.Application, 0, len(items))
	for _, item := range items {
		if seen[item.App.ID] {
			continue
		}
		seen[item.App.ID] = true
		out = append(out, item.App)
	}
	return out
}

func firstCategory(item model.Application) string {
	if len(item.Categories) == 0 {
		return "App"
	}
	return item.Categories[0]
}

func formatCount(value int) string {
	if value < 1000 {
		return fmt.Sprintf("%d", value)
	}
	text := fmt.Sprintf("%d", value)
	first := len(text) % 3
	if first == 0 {
		first = 3
	}
	var out strings.Builder
	out.WriteString(text[:first])
	for i := first; i < len(text); i += 3 {
		out.WriteByte(',')
		out.WriteString(text[i : i+3])
	}
	return out.String()
}

func choose[T any](condition bool, yes, no T) T {
	if condition {
		return yes
	}
	return no
}

func chooseColor(condition bool, yes, no color.NRGBA) color.NRGBA {
	if condition {
		return yes
	}
	return no
}

func transactionError(err error, output string) string {
	message := err.Error()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if len(line) > 180 {
			line = line[:177] + "..."
		}
		return message + "  ·  " + line
	}
	return message
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
