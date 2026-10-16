package ui

import (
	"bytes"
	"context"
	"crypto/sha256"
	hexenc "encoding/hex"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gioui.org/op/paint"
	_ "golang.org/x/image/webp"

	"github.com/Ryoku-dev/ryomarket/internal/metadata"
	"github.com/Ryoku-dev/ryomarket/internal/model"
)

type imageResult struct {
	appID string
	asset string
	index int
	op    paint.ImageOp
	ok    bool
}

var mediaSlots = make(chan struct{}, 4)
var metadataSlots = make(chan struct{}, 4)

func (a *App) loadIcon(item model.Application) {
	appID := item.ID
	if a.imageAssetSettled(appID, "icon", 0) {
		return
	}
	flatpakID := flatpakID(item)
	urls := make([]string, 0, 4)
	if data, ok := a.metadataFor(appID); ok && data.Icon != "" {
		urls = append(urls, data.Icon)
	}
	if strings.HasPrefix(item.Icon, "https://") || strings.HasPrefix(item.Icon, "http://") {
		urls = append(urls, item.Icon)
	}
	if flatpakID != "" {
		urls = append(urls, flathubIconURLs(flatpakID)...)
	}
	a.loadImageAssetWithLocal(appID, "icon", 0, urls, func() (paint.ImageOp, bool) {
		if flatpakID == "" {
			return paint.ImageOp{}, false
		}
		return localFlatpakIcon(flatpakID)
	})
}

func (a *App) loadApplicationScreenshots(item model.Application) {
	for index, rawURL := range item.Screenshots {
		if index >= 4 {
			break
		}
		a.loadImageAsset(item.ID, "screenshot", index, []string{rawURL})
	}
}

func (a *App) loadMediaFromMetadata(appID string, data metadata.FlathubApp) {
	iconURLs := []string{}
	if data.Icon != "" {
		iconURLs = append(iconURLs, data.Icon)
	}
	if data.ID != "" {
		iconURLs = append(iconURLs, flathubIconURLs(data.ID)...)
	}
	a.loadImageAsset(appID, "icon", 0, iconURLs)
	for index, shot := range data.Screenshots {
		if index >= 4 {
			break
		}
		a.loadImageAsset(appID, "screenshot", index, shot.URLs)
	}
}

func (a *App) imageAssetSettled(appID, asset string, index int) bool {
	key := imageAssetKey(appID, asset, index)
	a.mu.RLock()
	defer a.mu.RUnlock()
	cached := false
	if asset == "icon" {
		_, cached = a.images[appID]
	} else if shots, ok := a.screenshots[appID]; ok {
		_, cached = shots[index]
	}
	if cached || a.imageLoading[key] {
		return true
	}
	failedAt := a.imageFailedAt[key]
	return !failedAt.IsZero() && time.Since(failedAt) < 2*time.Minute
}

func (a *App) loadImageAsset(appID, asset string, index int, urls []string) {
	a.loadImageAssetWithLocal(appID, asset, index, urls, nil)
}

func (a *App) loadImageAssetWithLocal(appID, asset string, index int, urls []string, local func() (paint.ImageOp, bool)) {
	key := imageAssetKey(appID, asset, index)
	a.mu.Lock()
	cached := false
	if asset == "icon" {
		_, cached = a.images[appID]
	} else if shots, ok := a.screenshots[appID]; ok {
		_, cached = shots[index]
	}
	loading := a.imageLoading[key]
	failedAt := a.imageFailedAt[key]
	cooldown := !failedAt.IsZero() && time.Since(failedAt) < 2*time.Minute
	if !cached && !loading && !cooldown {
		a.imageLoading[key] = true
	}
	a.mu.Unlock()
	if cached || loading || cooldown {
		return
	}
	urls = uniqueHTTPURLs(urls)
	if len(urls) == 0 && local == nil {
		a.mu.Lock()
		delete(a.imageLoading, key)
		a.mu.Unlock()
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
		defer cancel()
		select {
		case mediaSlots <- struct{}{}:
			defer func() { <-mediaSlots }()
		case <-ctx.Done():
			a.imageCh <- imageResult{appID: appID, asset: asset, index: index, ok: false}
			return
		}
		if local != nil {
			if op, ok := local(); ok {
				a.imageCh <- imageResult{appID: appID, asset: asset, index: index, op: op, ok: true}
				a.win.Invalidate()
				return
			}
		}
		for _, rawURL := range urls {
			op, ok := fetchImage(ctx, rawURL)
			if ok {
				a.imageCh <- imageResult{appID: appID, asset: asset, index: index, op: op, ok: true}
				a.win.Invalidate()
				return
			}
		}
		a.imageCh <- imageResult{appID: appID, asset: asset, index: index, ok: false}
		a.win.Invalidate()
	}()
}

func (a *App) artwork(appID string) (paint.ImageOp, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	op, ok := a.images[appID]
	return op, ok
}

func (a *App) screenshot(appID string, index int) (paint.ImageOp, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	shots, ok := a.screenshots[appID]
	if !ok {
		return paint.ImageOp{}, false
	}
	op, ok := shots[index]
	return op, ok
}

func (a *App) screenshotCount(appID string) int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.screenshots[appID])
}

func imageAssetKey(appID, asset string, index int) string {
	return appID + ":" + asset + ":" + string(rune(index+'0'))
}

func uniqueHTTPURLs(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] || (!strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "http://")) {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func flatpakID(item model.Application) string {
	for _, variants := range item.Variants {
		for _, variant := range variants {
			if variant.Kind == model.ProviderFlatpak && strings.TrimSpace(variant.Package) != "" {
				return strings.TrimSpace(variant.Package)
			}
		}
	}
	return ""
}

func flatpakArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

func flathubIconURLs(appID string) []string {
	arches := []string{flatpakArch()}
	if arches[0] != "x86_64" {
		arches = append(arches, "x86_64")
	}
	urls := make([]string, 0, len(arches)*2)
	for _, arch := range arches {
		urls = append(urls,
			"https://dl.flathub.org/repo/appstream/"+arch+"/icons/128x128/"+appID+".png",
			"https://flathub.org/repo/appstream/"+arch+"/icons/128x128/"+appID+".png",
		)
	}
	return urls
}

func localFlatpakIcon(appID string) (paint.ImageOp, bool) {
	home, _ := os.UserHomeDir()
	arch := flatpakArch()
	roots := []string{
		filepath.Join(home, ".local", "share", "flatpak", "appstream", "flathub", arch, "active", "icons"),
		filepath.Join(home, ".local", "share", "flatpak", "appstream2", "flathub", arch, "active", "icons"),
		filepath.Join("/var/lib/flatpak/appstream/flathub", arch, "active", "icons"),
		filepath.Join("/var/lib/flatpak/appstream2/flathub", arch, "active", "icons"),
	}
	for _, root := range roots {
		for _, size := range []string{"256x256", "128x128", "64x64"} {
			path := filepath.Join(root, size, appID+".png")
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			img, _, err := image.Decode(bytes.NewReader(data))
			if err == nil {
				return paint.NewImageOp(img), true
			}
		}
	}
	return paint.ImageOp{}, false
}

func fetchImage(ctx context.Context, rawURL string) (paint.ImageOp, bool) {
	if op, ok := cachedImage(rawURL); ok {
		return op, true
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return paint.ImageOp{}, false
	}
	req.Header.Set("Accept", "image/webp,image/png,image/jpeg,image/*;q=0.8")
	req.Header.Set("Referer", "https://flathub.org/")
	req.Header.Set("User-Agent", "Ryomarket (+https://github.com/Ryoku-dev)")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return paint.ImageOp{}, false
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return paint.ImageOp{}, false
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 10<<20))
	if err != nil || len(data) == 0 {
		return paint.ImageOp{}, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return paint.ImageOp{}, false
	}
	writeImageCache(rawURL, data)
	return paint.NewImageOp(img), true
}

func cachedImage(rawURL string) (paint.ImageOp, bool) {
	data, err := os.ReadFile(imageCachePath(rawURL))
	if err != nil {
		return paint.ImageOp{}, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return paint.ImageOp{}, false
	}
	return paint.NewImageOp(img), true
}

func writeImageCache(rawURL string, data []byte) {
	path := imageCachePath(rawURL)
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func imageCachePath(rawURL string) string {
	home, _ := os.UserHomeDir()
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		cacheHome = filepath.Join(home, ".cache")
	}
	sum := sha256.Sum256([]byte(rawURL))
	return filepath.Join(cacheHome, "ryomarket", "media", hexenc.EncodeToString(sum[:])+".img")
}
