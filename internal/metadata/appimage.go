package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type flexibleString string

func (s *flexibleString) UnmarshalJSON(data []byte) error {
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	switch value := value.(type) {
	case string:
		*s = flexibleString(value)
	case json.Number:
		*s = flexibleString(value.String())
	case bool:
		*s = flexibleString(fmt.Sprint(value))
	case nil:
		*s = ""
	default:
		*s = ""
	}
	return nil
}

type AppImageAuthor struct {
	Name flexibleString `json:"name"`
	URL  flexibleString `json:"url"`
}

type AppImageLink struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type AppImageItem struct {
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	Categories    []any            `json:"categories"`
	Authors       []AppImageAuthor `json:"authors"`
	License       string           `json:"license"`
	Links         []AppImageLink   `json:"links"`
	Icons         []string         `json:"icons"`
	Screenshots   []string         `json:"screenshots"`
	Libc          string           `json:"libc"`
	SelfContained bool             `json:"self_contained"`
	GlibcRequired string           `json:"glibc_required"`
}

type appImageFeed struct {
	Version int               `json:"version"`
	Items   []json.RawMessage `json:"items"`
}

type AppImageClient struct {
	HTTP *http.Client
	URL  string
}

func (c AppImageClient) All(ctx context.Context) ([]AppImageItem, error) {
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	feedURL := c.URL
	if feedURL == "" {
		feedURL = "https://appimage.github.io/feed.json"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Ryomarket (+https://github.com/Ryoku-dev)")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AppImage catalogue returned %s", res.Status)
	}
	var feed appImageFeed
	if err := json.NewDecoder(res.Body).Decode(&feed); err != nil {
		return nil, err
	}
	out := make([]AppImageItem, 0, len(feed.Items))
	for _, raw := range feed.Items {
		var item AppImageItem
		if json.Unmarshal(raw, &item) != nil {
			continue
		}
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			continue
		}
		item.Description = cleanHTML(item.Description)
		out = append(out, item)
	}
	return out, nil
}
