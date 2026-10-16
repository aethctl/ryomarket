package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var tags = regexp.MustCompile(`<[^>]+>`)

type FlathubClient struct {
	HTTP    *http.Client
	BaseURL string
}

type Screenshot struct {
	Caption string
	URLs    []string
}

type FlathubApp struct {
	ID            string
	Name          string
	Summary       string
	Description   string
	DeveloperName string
	Icon          string
	Categories    []string
	Screenshots   []Screenshot
	Releases      []struct {
		Version   string `json:"version"`
		Timestamp string `json:"timestamp"`
	}
}

type FlathubSearchApp struct {
	AppID         string
	Name          string
	Summary       string
	Description   string
	DeveloperName string
	Icon          string
	Categories    []string
	SubCategories []string
	Keywords      []string
	Verified      bool
}

type searchResponse struct {
	Hits       []searchHit `json:"hits"`
	Page       int         `json:"page"`
	TotalPages int         `json:"totalPages"`
	TotalHits  int         `json:"totalHits"`
}

type searchHit struct {
	ID                   string          `json:"id"`
	AppID                string          `json:"app_id"`
	Name                 string          `json:"name"`
	Summary              string          `json:"summary"`
	Description          string          `json:"description"`
	DeveloperName        string          `json:"developer_name"`
	Icon                 string          `json:"icon"`
	Keywords             []string        `json:"keywords"`
	MainCategories       json.RawMessage `json:"main_categories"`
	SubCategories        json.RawMessage `json:"sub_categories"`
	VerificationVerified bool            `json:"verification_verified"`
}

type flathubWire struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Summary       string            `json:"summary"`
	Description   string            `json:"description"`
	DeveloperName string            `json:"developer_name"`
	Icon          json.RawMessage   `json:"icon"`
	Icons         []json.RawMessage `json:"icons"`
	Categories    []string          `json:"categories"`
	Screenshots   []json.RawMessage `json:"screenshots"`
	Releases      []struct {
		Version   string `json:"version"`
		Timestamp string `json:"timestamp"`
	} `json:"releases"`
}

func (c FlathubClient) App(ctx context.Context, id string) (FlathubApp, error) {
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://flathub.org/api/v2"
	}
	requestURL := baseURL + "/appstream/" + url.PathEscape(id) + "?locale=en"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return FlathubApp{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", "https://flathub.org")
	req.Header.Set("Referer", "https://flathub.org/")
	req.Header.Set("User-Agent", "Ryomarket (+https://github.com/Ryoku-dev)")
	res, err := client.Do(req)
	if err != nil {
		return FlathubApp{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return FlathubApp{}, fmt.Errorf("flathub returned %s", res.Status)
	}
	var wire flathubWire
	if err := json.NewDecoder(res.Body).Decode(&wire); err != nil {
		return FlathubApp{}, err
	}
	icon := firstURL(wire.Icon)
	if icon == "" {
		for _, raw := range wire.Icons {
			if icon = firstURL(raw); icon != "" {
				break
			}
		}
	}
	app := FlathubApp{
		ID:            wire.ID,
		Name:          wire.Name,
		Summary:       wire.Summary,
		Description:   cleanHTML(wire.Description),
		DeveloperName: wire.DeveloperName,
		Categories:    wire.Categories,
		Releases:      wire.Releases,
		Icon:          icon,
	}
	for _, raw := range wire.Screenshots {
		shot := parseScreenshot(raw)
		if len(shot.URLs) > 0 {
			app.Screenshots = append(app.Screenshots, shot)
		}
	}
	return app, nil
}

func (c FlathubClient) SearchAll(ctx context.Context) ([]FlathubSearchApp, int, error) {
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://flathub.org/api/v2"
	}

	apps := make([]FlathubSearchApp, 0, 3000)
	total := 0
	for page := 1; page <= 10; page++ {
		body, err := json.Marshal(map[string]any{
			"query":         "",
			"hits_per_page": 1000,
			"page":          page,
		})
		if err != nil {
			return nil, 0, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/search?locale=en", bytes.NewReader(body))
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Origin", "https://flathub.org")
		req.Header.Set("Referer", "https://flathub.org/")
		req.Header.Set("User-Agent", "Ryomarket (+https://github.com/Ryoku-dev)")

		res, err := client.Do(req)
		if err != nil {
			return nil, total, err
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			return nil, total, fmt.Errorf("flathub search returned %s", res.Status)
		}
		var response searchResponse
		err = json.NewDecoder(res.Body).Decode(&response)
		res.Body.Close()
		if err != nil {
			return nil, total, err
		}
		if response.TotalHits > total {
			total = response.TotalHits
		}
		for _, hit := range response.Hits {
			appID := strings.TrimSpace(hit.AppID)
			if appID == "" {
				appID = strings.TrimSpace(hit.ID)
			}
			name := strings.TrimSpace(hit.Name)
			if appID == "" || name == "" {
				continue
			}
			apps = append(apps, FlathubSearchApp{
				AppID:         appID,
				Name:          name,
				Summary:       cleanHTML(hit.Summary),
				Description:   cleanHTML(hit.Description),
				DeveloperName: strings.TrimSpace(hit.DeveloperName),
				Icon:          strings.TrimSpace(hit.Icon),
				Categories:    searchCategories(hit.MainCategories),
				SubCategories: searchCategories(hit.SubCategories),
				Keywords:      hit.Keywords,
				Verified:      hit.VerificationVerified,
			})
		}
		if response.TotalPages <= page || len(response.Hits) == 0 {
			break
		}
	}
	return apps, total, nil
}

func searchCategories(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var single string
	if json.Unmarshal(raw, &single) == nil && strings.TrimSpace(single) != "" {
		return []string{single}
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		out := many[:0]
		for _, value := range many {
			if value = strings.TrimSpace(value); value != "" {
				out = append(out, value)
			}
		}
		return out
	}
	return nil
}

func parseScreenshot(raw json.RawMessage) Screenshot {
	var value any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return Screenshot{}
	}
	shot := Screenshot{}
	if object, ok := value.(map[string]any); ok {
		if caption, ok := object["caption"].(string); ok {
			shot.Caption = cleanHTML(caption)
		}
		if sizes, ok := object["sizes"].([]any); ok {
			type candidate struct {
				url  string
				area int
			}
			candidates := make([]candidate, 0, len(sizes))
			for _, value := range sizes {
				size, ok := value.(map[string]any)
				if !ok {
					continue
				}
				src, _ := size["src"].(string)
				if !isHTTPURL(src) {
					continue
				}
				w := numberValue(size["width"])
				h := numberValue(size["height"])
				candidates = append(candidates, candidate{url: src, area: w * h})
			}
			sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].area > candidates[j].area })
			for _, candidate := range candidates {
				shot.URLs = append(shot.URLs, candidate.url)
			}
		}
	}
	seen := map[string]bool{}
	for _, url := range shot.URLs {
		seen[url] = true
	}
	for _, url := range collectURLs(value) {
		if !seen[url] {
			seen[url] = true
			shot.URLs = append(shot.URLs, url)
		}
	}
	return shot
}

func numberValue(value any) int {
	switch value := value.(type) {
	case string:
		n, _ := strconv.Atoi(value)
		return n
	case float64:
		return int(value)
	default:
		return 0
	}
}

func firstURL(raw json.RawMessage) string {
	var value any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return ""
	}
	urls := collectURLs(value)
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

func collectURLs(value any) []string {
	seen := map[string]bool{}
	out := []string{}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case string:
			if isHTTPURL(typed) && !seen[typed] {
				seen[typed] = true
				out = append(out, typed)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.SliceStable(keys, func(i, j int) bool {
				return mediaKeyRank(keys[i]) < mediaKeyRank(keys[j])
			})
			for _, key := range keys {
				walk(typed[key])
			}
		}
	}
	walk(value)
	return out
}

func mediaKeyRank(key string) int {
	key = strings.ToLower(key)
	switch {
	case strings.Contains(key, "1280") || strings.Contains(key, "1248") || strings.Contains(key, "1920"):
		return 0
	case strings.Contains(key, "752") || strings.Contains(key, "624"):
		return 1
	case strings.Contains(key, "512") || strings.Contains(key, "256"):
		return 2
	case strings.Contains(key, "128"):
		return 3
	case key == "url" || key == "src" || key == "source" || key == "image":
		return 4
	default:
		return 10
	}
}

func isHTTPURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://")
}

func cleanHTML(value string) string {
	value = strings.ReplaceAll(value, "</p>", "\n\n")
	value = strings.ReplaceAll(value, "</li>", "\n")
	value = strings.ReplaceAll(value, "<br>", "\n")
	value = strings.ReplaceAll(value, "<br/>", "\n")
	value = strings.ReplaceAll(value, "<br />", "\n")
	value = tags.ReplaceAllString(value, "")
	value = html.UnescapeString(value)

	paragraphs := strings.Split(value, "\n")
	out := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		paragraph = strings.Join(strings.Fields(paragraph), " ")
		if paragraph != "" {
			out = append(out, paragraph)
		}
	}
	return strings.Join(out, "\n\n")
}
