package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppImageClientAll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":1,"items":[{"name":"Example","description":"<p>Example app</p>","categories":["Utility"],"authors":[{"name":"Dev"}],"links":[{"type":"GitHub","url":"owner/example"}],"icons":["Example/icons/128x128/example.png"],"screenshots":["Example/screenshot.png"],"glibc_required":"2.31"}]}`))
	}))
	defer server.Close()

	items, err := (AppImageClient{URL: server.URL}).All(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "Example" || items[0].Description != "Example app" {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestAppImageClientToleratesMalformedAuthorFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":1,"items":[{"name":"Numeric Author","description":"Works","authors":[{"name":123,"url":false}],"links":[{"type":"GitHub","url":"owner/numeric"}]},{"name":"Broken","icons":17},{"name":"Good","links":[{"type":"GitHub","url":"owner/good"}]}]}`))
	}))
	defer server.Close()

	items, err := (AppImageClient{URL: server.URL}).All(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected malformed item to be skipped without failing feed, got %#v", items)
	}
	if got := string(items[0].Authors[0].Name); got != "123" {
		t.Fatalf("unexpected flexible author name %q", got)
	}
}
