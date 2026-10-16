package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
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
	viewDiscover  = "discover"
	viewInstalled = "installed"
	viewUpdates   = "updates"
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
	win      *app.Window
	theme    *material.Theme
	catalog  *catalog.Catalog
	service  *service.Service
	state    *state.Store
	host     platform.Info
	selected model.Distro

	view         string
	category     string
	source       string
	selectedID   string
	lastQuery    string
	query        widget.Editor
	list         layout.List
	detailList   layout.List
	categoryList layout.List

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

	variantSelection map[string]int
	pending          *provider.Plan
	busy             bool
	checking         map[string]bool
	metadata         map[string]metadata.FlathubApp
	metadataLoading  map[string]bool
	metadataFailedAt map[string]time.Time
	images           map[string]paint.ImageOp
	screenshots      map[string]map[int]paint.ImageOp
	imageLoading     map[string]bool
	imageFailedAt    map[string]time.Time
	themeKey         string
	nextThemeCheck   time.Time
	installed        []service.Installed
	installedApps    []model.Application
	updates          []service.Update
	updateApps       []model.Application
	scanBusy         bool
	status           string
	statusBad        bool
	catalogLoading   bool
	catalogPending   int
	catalogTotal     int
	catalogEpoch     uint64
	discoverCacheKey string
	discoverApps     []model.Application
	discoverCats     []string
	discoverCount    int
	sourceCounts     map[string]int
	installedIDs     map[string]struct{}

	txCh       chan txResult
	scanCh     chan scanResult
	metadataCh chan metadataResult
	imageCh    chan imageResult
	catalogCh  chan catalogResult
	mu         sync.RWMutex
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
		app.Title("Ryomarket"),
		app.Size(unit.Dp(1280), unit.Dp(820)),
		app.MinSize(unit.Dp(980), unit.Dp(680)),
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
	a.themeKey = themeStateKey()
	a.query.SingleLine = true
	for _, key := range []string{viewDiscover, viewInstalled, viewUpdates} {
		a.navButtons[key] = new(widget.Clickable)
	}
	if selected != model.DistroUnknown {
		a.startScan(false)
	}
	a.startCatalogRefresh()

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
				a.status = transactionError(result.err, result.output)
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
	if a.nextThemeCheck.IsZero() || !gtx.Now.Before(a.nextThemeCheck) {
		a.nextThemeCheck = gtx.Now.Add(time.Second)
		key := themeStateKey()
		if key != a.themeKey {
			colors = loadPalette()
			a.theme = newTheme()
			a.themeKey = key
		}
	}
	gtx.Execute(op.InvalidateCmd{At: a.nextThemeCheck})
}

func (a *App) layout(gtx layout.Context) layout.Dimensions {
	a.refreshTheme(gtx)
	fill(gtx, colors.bg)
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widget.Border{Color: colors.line, CornerRadius: unit.Dp(10), Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return roundedBackground(gtx, colors.bg, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Dp(unit.Dp(268))
						gtx.Constraints.Max.X = gtx.Constraints.Min.X
						return a.rail(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Dp(unit.Dp(1))
						gtx.Constraints.Max.X = gtx.Constraints.Min.X
						fill(gtx, colors.line)
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}),
					layout.Flexed(1, a.content),
				)
			})
		})
	})
}

func (a *App) rail(gtx layout.Context) layout.Dimensions {
	fill(gtx, colors.rail)
	return layout.Inset{Top: unit.Dp(18), Bottom: unit.Dp(14), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.brandBlock),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(a.searchBox),
			layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
			layout.Rigid(a.railLabel("BROWSE")),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.navItem(gtx, viewDiscover, "Discover", "発見", "")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				sub := fmt.Sprintf("%d detected", len(a.installed))
				if a.scanBusy && len(a.installed) == 0 {
					sub = "Scanning this system"
				}
				return a.navItem(gtx, viewInstalled, "Installed", "導入", sub)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				sub := fmt.Sprintf("%d updates available", len(a.updates))
				return a.navItem(gtx, viewUpdates, "Updates", "更新", sub)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
			layout.Rigid(a.railLabel("SOURCES")),
			layout.Rigid(a.sourceRail),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
			layout.Rigid(a.railLabel("SYSTEM")),
			layout.Rigid(a.systemCard),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.refreshButton.Clicked(gtx) && !a.scanBusy && a.host.Distro != model.DistroUnknown {
					a.startScan(a.view == viewUpdates)
				}
				text := "REFRESH STATUS"
				if a.scanBusy {
					text = "SCANNING…"
				}
				style := button(a.theme, &a.refreshButton, text, false)
				style.TextSize = unit.Sp(10)
				style.Inset = layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(11), Right: unit.Dp(11)}
				gtx.Constraints.Min.X = 0
				return style.Layout(gtx)
			}),
		)
	})
}

func (a *App) brandBlock(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(42)), gtx.Dp(unit.Dp(42)))
			gtx.Constraints.Max = gtx.Constraints.Min
			return widget.Border{Color: colors.line, CornerRadius: unit.Dp(6), Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return roundedBackground(gtx, colors.surface2, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min = image.Point{}
						l := jpLabel(a.theme, unit.Sp(21), "力", colors.accent)
						l.Font.Weight = font.Bold
						l.Alignment = text.Middle
						return l.Layout(gtx)
					})
				})
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(mediumLabel(a.theme, unit.Sp(16), "RYOKU", colors.bright).Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout),
				layout.Rigid(mediumLabel(a.theme, unit.Sp(9), "MARKET", colors.subtle).Layout),
			)
		}),
	)
}

func (a *App) searchBox(gtx layout.Context) layout.Dimensions {
	hint := "⌕  Search apps"
	count := a.sourceCount(a.source)
	if count > 0 {
		hint = fmt.Sprintf("⌕  Search %s apps", formatCount(count))
	}
	return boxed(gtx, colors.surface, colors.line, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
		ed := material.Editor(a.theme, &a.query, hint)
		ed.TextSize = unit.Sp(13)
		ed.Color = colors.bright
		ed.HintColor = colors.dim
		ed.SelectionColor = withAlpha(colors.accent, 90)
		return ed.Layout(gtx)
	})
}

func (a *App) railLabel(value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(2), Bottom: unit.Dp(7)}.Layout(gtx, mediumLabel(a.theme, unit.Sp(9), value, colors.dim).Layout)
	}
}

func (a *App) navItem(gtx layout.Context, key, name, jp, subtitle string) layout.Dimensions {
	click := a.navButtons[key]
	if click.Clicked(gtx) {
		a.view = key
		a.selectedID = ""
		a.pending = nil
		a.list.Position = layout.Position{}
		if key == viewInstalled || key == viewUpdates {
			a.startScan(key == viewUpdates)
		}
	}
	active := a.view == key
	fg := colors.cream
	sub := colors.dim
	jpColor := colors.dim
	bg := color.NRGBA{}
	if active {
		bg = colors.bone
		fg = hex(0x000000)
		sub = hex(0x000000)
		jpColor = hex(0x000000)
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		content := func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(10), Right: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						children := []layout.FlexChild{
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := mediumLabel(a.theme, unit.Sp(15), name, fg)
								if active {
									l.Color = hex(0x000000)
									l.Font.Weight = font.Bold
								}
								return l.Layout(gtx)
							}),
						}
						if strings.TrimSpace(subtitle) != "" {
							children = append(children,
								layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := mediumLabel(a.theme, unit.Sp(10), subtitle, sub)
									if active {
										l.Color = hex(0x000000)
										l.Font.Weight = font.Medium
									}
									l.MaxLines = 1
									return l.Layout(gtx)
								}),
							)
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := jpLabel(a.theme, unit.Sp(11), jp, jpColor)
						if active {
							l.Color = hex(0x000000)
							l.Font.Weight = font.Bold
						}
						return l.Layout(gtx)
					}),
				)
			})
		}
		if active {
			return roundedBackground(gtx, bg, unit.Dp(6), content)
		}
		if click.Hovered() {
			return roundedBackground(gtx, colors.surface, unit.Dp(6), content)
		}
		return content(gtx)
	})
}

func (a *App) content(gtx layout.Context) layout.Dimensions {
	query := a.query.Text()
	if query != a.lastQuery && strings.TrimSpace(query) != "" && a.view != viewDiscover {
		a.view = viewDiscover
		a.selectedID = ""
		a.list.Position = layout.Position{}
	}
	return layout.Inset{Top: unit.Dp(18), Bottom: unit.Dp(0), Left: unit.Dp(28), Right: unit.Dp(28)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(a.header),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Flexed(1, a.body),
			layout.Rigid(a.footer),
		)
	})
}

func (a *App) header(gtx layout.Context) layout.Dimensions {
	if a.selectedID != "" {
		return label(a.theme, unit.Sp(9), "力  SOFTWARE", colors.dim).Layout(gtx)
	}

	title := "Discover"
	sub := "One software catalogue across every Ryoku distro."
	if a.view == viewInstalled {
		title, sub = "Installed", "Software Ryomarket can identify on this system."
	} else if a.view == viewUpdates {
		title, sub = "Updates", "Provider-aware updates without duplicate listings."
	}
	count := a.headerCount()
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(label(a.theme, unit.Sp(9), "力  BROWSE", colors.dim).Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
				layout.Rigid(displayLabel(a.theme, unit.Sp(30), title, colors.bright).Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
				layout.Rigid(label(a.theme, unit.Sp(13), sub, colors.subtle).Layout),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
					layout.Rigid(label(a.theme, unit.Sp(22), fmt.Sprintf("%d", count), colors.bright).Layout),
					layout.Rigid(label(a.theme, unit.Sp(8), choose(count == 1, "APP", "APPS"), colors.dim).Layout),
				)
			})
		}),
	)
}

func (a *App) ensureDiscoverCache() {
	key := fmt.Sprintf("%d\x00%s\x00%s\x00%s", a.catalogEpoch, a.source, a.category, a.query.Text())
	if key == a.discoverCacheKey {
		return
	}
	result := a.catalog.BrowseAllByProviders(a.query.Text(), a.category, sourceKinds(a.source))
	a.discoverCacheKey = key
	a.discoverApps = result.Apps
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
		if appItem, ok := a.catalog.App(a.selectedID); ok {
			return a.detail(gtx, appItem)
		}
		a.selectedID = ""
	}
	switch a.view {
	case viewInstalled:
		return a.installedView(gtx)
	case viewUpdates:
		return a.updatesView(gtx)
	default:
		return a.discover(gtx)
	}
}

func (a *App) discover(gtx layout.Context) layout.Dimensions {
	query := a.query.Text()
	if query != a.lastQuery {
		a.lastQuery = query
		a.list.Position = layout.Position{}
	}
	a.ensureDiscoverCache()
	cats := a.discoverCats
	apps := a.discoverApps
	for _, cat := range cats {
		if a.categoryButtons[cat] == nil {
			a.categoryButtons[cat] = new(widget.Clickable)
		}
	}
	if len(apps) == 0 {
		return a.empty(gtx, "NO MATCHES", "Try another search or category.")
	}

	showHero := strings.TrimSpace(query) == "" && strings.EqualFold(a.category, "All") && a.source == sourceAll
	children := make([]layout.FlexChild, 0, 6)
	if showHero {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.hero(gtx, apps[0]) }),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		)
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(38))
			return a.categoryList.Layout(gtx, len(cats), func(gtx layout.Context, index int) layout.Dimensions {
				return a.categoryChip(gtx, cats[index])
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return a.appGrid(gtx, apps) }),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) hero(gtx layout.Context, item model.Application) layout.Dimensions {
	a.loadMetadata(item)
	display := a.enriched(item)
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(232))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y

	if a.heroDetailsButton.Clicked(gtx) {
		a.openDetail(item)
	}
	return widget.Border{Color: colors.line, CornerRadius: unit.Dp(6), Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return roundedBackground(gtx, colors.surface, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(0.54, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(20), Left: unit.Dp(24), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						variantIndex := a.defaultVariant(item)
						providerName := "No host source"
						if variants := item.Variants[a.selected]; variantIndex >= 0 && variantIndex < len(variants) {
							providerName = variantName(variants[variantIndex])
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(label(a.theme, unit.Sp(8), strings.ToUpper(firstCategory(display)), colors.dim).Layout),
							layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
							layout.Rigid(displayLabel(a.theme, unit.Sp(29), display.Name, colors.bright).Layout),
							layout.Rigid(layout.Spacer{Height: unit.Dp(7)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := label(a.theme, unit.Sp(12), display.Summary, colors.subtle)
								l.MaxLines = 2
								return l.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(9)}.Layout),
							layout.Rigid(label(a.theme, unit.Sp(8), "•  "+strings.ToUpper(providerName), colors.dim).Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.heroPrimary(gtx, item, variantIndex) }),
									layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
									layout.Rigid(button(a.theme, &a.heroDetailsButton, "VIEW DETAILS", false).Layout),
								)
							}),
						)
					})
				}),
				layout.Flexed(0.46, func(gtx layout.Context) layout.Dimensions {
					return a.heroMedia(gtx, item, display)
				}),
			)
		})
	})
}

func (a *App) heroPrimary(gtx layout.Context, item model.Application, variantIndex int) layout.Dimensions {
	if variantIndex < 0 || variantIndex >= len(item.Variants[a.selected]) {
		if a.heroPrimaryButton.Clicked(gtx) {
			a.openDetail(item)
		}
		return button(a.theme, &a.heroPrimaryButton, "BROWSE", true).Layout(gtx)
	}
	if !a.service.VariantAvailable(a.selected, item.Variants[a.selected][variantIndex]) {
		if a.heroPrimaryButton.Clicked(gtx) {
			a.openDetail(item)
		}
		return button(a.theme, &a.heroPrimaryButton, "VIEW SOURCES", true).Layout(gtx)
	}
	installed := a.service.VariantInstalled(item.ID, variantIndex)
	text := "INSTALL"
	if installed {
		text = "INSTALLED"
	}
	if a.busy {
		text = "WORKING…"
	}
	if a.heroPrimaryButton.Clicked(gtx) && !a.busy {
		if installed {
			a.openDetail(item)
		} else {
			plan, err := a.service.Plan(item, a.selected, variantIndex, provider.Install)
			if err != nil {
				a.status, a.statusBad = err.Error(), true
			} else {
				a.openDetail(item)
				a.pending = &plan
			}
		}
	}
	return button(a.theme, &a.heroPrimaryButton, text, true).Layout(gtx)
}

func (a *App) categoryChip(gtx layout.Context, category string) layout.Dimensions {
	click := a.categoryButtons[category]
	if click.Clicked(gtx) {
		a.category = category
		a.list.Position = layout.Position{}
	}
	active := strings.EqualFold(a.category, category)
	return layout.Inset{Right: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		style := button(a.theme, click, strings.ToUpper(category), active)
		style.TextSize = unit.Sp(8)
		style.Inset = layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(12), Right: unit.Dp(12)}
		if !active {
			style.Background = colors.surface
			style.Color = colors.dim
		}
		return style.Layout(gtx)
	})
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
	display := a.enriched(item)
	click := a.cardButtons[item.ID]
	if click == nil {
		click = new(widget.Clickable)
		a.cardButtons[item.ID] = click
	}
	if click.Clicked(gtx) {
		a.openDetail(item)
	}
	gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(240))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	border := colors.line
	if click.Hovered() {
		border = colors.strong
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widget.Border{Color: border, CornerRadius: unit.Dp(6), Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return roundedBackground(gtx, colors.surface, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(132))
						gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
						return a.posterArt(gtx, display, false)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(9), Left: unit.Dp(11), Right: unit.Dp(11)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							providerName := a.providerName(item)
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											l := displayLabel(a.theme, unit.Sp(15), display.Name, colors.bright)
											l.MaxLines = 1
											return l.Layout(gtx)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return boxed(gtx, colors.surface2, colors.line, unit.Dp(4), mediumLabel(a.theme, unit.Sp(7), scopeSummary(item), colors.subtle).Layout)
										}),
									)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := label(a.theme, unit.Sp(10), display.Summary, colors.subtle)
									l.MaxLines = 2
									return l.Layout(gtx)
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(label(a.theme, unit.Sp(8), strings.ToUpper(providerName), colors.dim).Layout),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if !a.appInstalled(item.ID) {
												return layout.Dimensions{}
											}
											return boxed(gtx, colors.surface2, colors.strong, unit.Dp(4), label(a.theme, unit.Sp(7), "INSTALLED", colors.subtle).Layout)
										}),
									)
								}),
							)
						})
					}),
				)
			})
		})
	})
}

func (a *App) posterArt(gtx layout.Context, item model.Application, large bool) layout.Dimensions {
	icon := unit.Dp(76)
	circle := unit.Dp(104)
	fontSize := unit.Sp(34)
	if large {
		icon = unit.Dp(128)
		circle = unit.Dp(176)
		fontSize = unit.Sp(54)
	}
	return roundedBackground(gtx, colors.surface2, unit.Dp(5), func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if imageOp, ok := a.artwork(item.ID); ok {
				gtx.Constraints.Min = image.Pt(gtx.Dp(icon), gtx.Dp(icon))
				gtx.Constraints.Max = gtx.Constraints.Min
				im := widget.Image{Src: imageOp, Fit: widget.Contain, Position: layout.Center}
				return im.Layout(gtx)
			}
			gtx.Constraints.Min = image.Pt(gtx.Dp(circle), gtx.Dp(circle))
			gtx.Constraints.Max = gtx.Constraints.Min
			return widget.Border{Color: colors.line, CornerRadius: circle / 2, Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = image.Point{}
					l := displayLabel(a.theme, fontSize, initials(item.Name), colors.bright)
					l.Alignment = text.Middle
					return l.Layout(gtx)
				})
			})
		})
	})
}

func (a *App) heroMedia(gtx layout.Context, item model.Application, display model.Application) layout.Dimensions {
	if imageOp, ok := a.screenshot(item.ID, 0); ok {
		return roundedBackground(gtx, colors.surface2, unit.Dp(5), func(gtx layout.Context) layout.Dimensions {
			im := widget.Image{Src: imageOp, Fit: widget.Cover, Position: layout.Center}
			return im.Layout(gtx)
		})
	}
	return a.posterArt(gtx, display, true)
}

func (a *App) previewGallery(gtx layout.Context, item model.Application) layout.Dimensions {
	count := len(item.Screenshots)
	source := "APP PREVIEW"
	if info, ok := a.metadataFor(item.ID); ok && len(info.Screenshots) > 0 {
		count = len(info.Screenshots)
		source = "PREVIEW  ·  FLATHUB APPSTREAM"
	}
	if count == 0 {
		return layout.Dimensions{}
	}
	if count > 3 {
		count = 3
	}
	return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(20)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(label(a.theme, unit.Sp(8), source, colors.dim).Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, count)
				for index := 0; index < count; index++ {
					i := index
					children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						right := unit.Dp(0)
						if i < count-1 {
							right = unit.Dp(8)
						}
						return layout.Inset{Right: right}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(172))
							gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
							return widget.Border{Color: colors.line, CornerRadius: unit.Dp(6), Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return roundedBackground(gtx, colors.surface2, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
									if imageOp, loaded := a.screenshot(item.ID, i); loaded {
										im := widget.Image{Src: imageOp, Fit: widget.Cover, Position: layout.Center}
										return im.Layout(gtx)
									}
									return layout.Center.Layout(gtx, label(a.theme, unit.Sp(9), "LOADING PREVIEW…", colors.dim).Layout)
								})
							})
						})
					}))
				}
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
			}),
		)
	})
}

func (a *App) monogram(gtx layout.Context, name string, size unit.Dp) layout.Dimensions {
	gtx.Constraints.Min = image.Pt(gtx.Dp(size), gtx.Dp(size))
	gtx.Constraints.Max = gtx.Constraints.Min
	return roundedBackground(gtx, colors.surface2, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
		l := displayLabel(a.theme, unit.Sp(float32(size)*0.31), initials(name), colors.bright)
		l.Alignment = text.Middle
		return l.Layout(gtx)
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
		status = "SYNCING CATALOGUE"
		statusColor = colors.subtle
	} else if a.scanBusy {
		status = "SCANNING"
		statusColor = colors.subtle
	} else if a.busy {
		status = "WORKING"
		statusColor = colors.subtle
	} else if a.statusBad && a.status != "" {
		status = a.status
		statusColor = colors.bad
	}
	if status == "" {
		return layout.Dimensions{}
	}
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := label(a.theme, unit.Sp(8), status, statusColor)
		l.MaxLines = 1
		return l.Layout(gtx)
	})
}

func (a *App) detail(gtx layout.Context, item model.Application) layout.Dimensions {
	a.loadMetadata(item)
	display := a.enriched(item)
	variants := item.Variants[a.selected]
	selected, selectedSet := a.variantSelection[item.ID]
	if !selectedSet || selected < 0 || selected >= len(variants) {
		selected = a.defaultVariant(item)
		a.variantSelection[item.ID] = selected
	}
	for i := range variants {
		key := fmt.Sprintf("%s:%s:%d", item.ID, a.selected, i)
		if a.providerButtons[key] == nil {
			a.providerButtons[key] = new(widget.Clickable)
		}
	}

	return a.detailList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.backButton.Clicked(gtx) {
					a.selectedID = ""
					a.pending = nil
				}
				style := button(a.theme, &a.backButton, "←  BACK", false)
				style.TextSize = unit.Sp(9)
				style.Inset = layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(10), Right: unit.Dp(10)}
				return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, style.Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(216))
				gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
				return widget.Border{Color: colors.line, CornerRadius: unit.Dp(6), Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return roundedBackground(gtx, colors.surface, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(0.38, func(gtx layout.Context) layout.Dimensions {
								return a.posterArt(gtx, display, true)
							}),
							layout.Flexed(0.62, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: unit.Dp(24), Bottom: unit.Dp(22), Left: unit.Dp(24), Right: unit.Dp(24)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(label(a.theme, unit.Sp(8), strings.ToUpper(firstCategory(display)), colors.dim).Layout),
										layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
										layout.Rigid(displayLabel(a.theme, unit.Sp(28), display.Name, colors.bright).Layout),
										layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											l := label(a.theme, unit.Sp(12), display.Summary, colors.subtle)
											l.MaxLines = 2
											return l.Layout(gtx)
										}),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											meta := strings.TrimSpace(display.Developer)
											if meta == "" {
												meta = a.providerName(item)
											}
											if info, ok := a.metadataFor(item.ID); ok && len(info.Releases) > 0 && info.Releases[0].Version != "" {
												meta += "  ·  " + info.Releases[0].Version
											}
											return label(a.theme, unit.Sp(8), strings.ToUpper(meta), colors.dim).Layout(gtx)
										}),
									)
								})
							}),
						)
					})
				})
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return boxed(gtx, colors.surface, colors.line, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
					l := label(a.theme, unit.Sp(12), display.Description, colors.cream)
					l.MaxLines = 14
					return l.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.previewGallery(gtx, item)
			}),
			layout.Rigid(label(a.theme, unit.Sp(8), "INSTALLATION SOURCE", colors.dim).Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(variants))
				for i := range variants {
					idx := i
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.providerRow(gtx, item, idx, selected == idx)
						})
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.actionArea(gtx, item, selected) }),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.pending == nil || a.pending.AppID != item.ID {
					return layout.Dimensions{}
				}
				return a.planPanel(gtx, *a.pending)
			}),
		)
	})
}

func (a *App) providerRow(gtx layout.Context, item model.Application, index int, active bool) layout.Dimensions {
	variant := item.Variants[a.selected][index]
	key := fmt.Sprintf("%s:%s:%d", item.ID, a.selected, index)
	click := a.providerButtons[key]
	if click.Clicked(gtx) && !a.busy {
		a.variantSelection[item.ID] = index
		a.pending = nil
	}
	border := colors.line
	bg := colors.surface2
	if active {
		border = colors.strong
		bg = colors.raised
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return boxed(gtx, bg, border, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					mark := "○"
					c := colors.dim
					if active {
						mark, c = "●", colors.accent
					}
					return label(a.theme, unit.Sp(12), mark, c).Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					name := variantName(variant)
					detail := variant.Package
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(label(a.theme, unit.Sp(14), name, colors.bright).Layout),
						layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							l := label(a.theme, unit.Sp(10), detail, colors.dim)
							l.MaxLines = 1
							return l.Layout(gtx)
						}),
					)
				}),
			)
		})
	})
}

func (a *App) actionArea(gtx layout.Context, item model.Application, selected int) layout.Dimensions {
	if selected < 0 || selected >= len(item.Variants[a.selected]) {
		return a.empty(gtx, "NO INSTALL SOURCE", "Available elsewhere as "+scopeSummary(item)+", but no provider matches "+a.host.Distro.DisplayName()+" on this machine.")
	}
	if a.busy {
		return a.compactActionButton(gtx, "WORKING…", false)
	}
	if a.isChecking(item.ID) {
		return a.compactActionButton(gtx, "CHECKING INSTALLATION…", false)
	}

	variant := item.Variants[a.selected][selected]
	if variant.Kind == model.ProviderNixConfig && a.host.Distro == model.DistroNixOS {
		return a.nixConfigGuide(gtx, variant)
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
	recommended := item.RecommendedVariant(a.selected)
	if recommended >= 0 && recommended < len(variants) && a.service.VariantAvailable(a.selected, variants[recommended]) {
		return recommended
	}
	for i, variant := range variants {
		if a.service.VariantAvailable(a.selected, variant) {
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
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		}
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

func (a *App) installedView(gtx layout.Context) layout.Dimensions {
	if a.scanBusy && len(a.installed) == 0 {
		return a.empty(gtx, "SCANNING", "Checking native repositories, Flatpak, portable apps and Nix providers.")
	}
	if len(a.installed) == 0 {
		return a.empty(gtx, "NOTHING DETECTED", "Refresh to scan Ryomarket's catalogue against this system.")
	}
	return a.appGrid(gtx, a.installedApps)
}

func (a *App) updatesView(gtx layout.Context) layout.Dimensions {
	if a.scanBusy {
		return a.empty(gtx, "CHECKING PROVIDERS", "Looking for updates without modifying package databases.")
	}
	if len(a.updates) == 0 {
		return a.empty(gtx, "UP TO DATE", "No provider reported an application update.")
	}
	return a.appGrid(gtx, a.updateApps)
}

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
	if a.scanBusy || a.host.Distro == model.DistroUnknown {
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
		data, err := (metadata.FlathubClient{}).App(ctx, flatpakID)
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
