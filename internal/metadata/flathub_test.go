package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFlathubApp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appstream/org.example.App" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("locale") != "en" {
			t.Fatalf("missing locale query")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"org.example.App","name":"Example","summary":"Useful","description":"<p>First paragraph.</p><p>Second &amp; better.</p>","developer_name":"Example Dev","icon":"https://example.test/icon.png","categories":["Utility"],"screenshots":[{"624x351":"https://example.test/shot.png","caption":"Main view"}],"releases":[{"version":"2.0","timestamp":"1"}]}`))
	}))
	defer server.Close()

	client := FlathubClient{BaseURL: server.URL}
	app, err := client.App(context.Background(), "org.example.App")
	if err != nil {
		t.Fatal(err)
	}
	if app.Name != "Example" || app.Description != "First paragraph.\n\nSecond & better." {
		t.Fatalf("unexpected app metadata: %#v", app)
	}
	if app.Icon != "https://example.test/icon.png" || len(app.Screenshots) != 1 || app.Screenshots[0].URLs[0] != "https://example.test/shot.png" {
		t.Fatalf("unexpected media metadata: %#v", app)
	}
}

func TestFlathubAppAcceptsCurrentMediaSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"org.example.App","name":"Example","icon":null,"icons":[{"url":"https://example.test/icon-128.png","width":128,"height":128}],"screenshots":[{"caption":"Main view","sizes":[{"width":"624","height":"351","scale":"1x","src":"https://example.test/shot-small.png"},{"width":"1280","height":"720","scale":"1x","src":"https://example.test/shot-large.png"}]}]}`))
	}))
	defer server.Close()

	app, err := (FlathubClient{BaseURL: server.URL}).App(context.Background(), "org.example.App")
	if err != nil {
		t.Fatal(err)
	}
	if app.Icon != "https://example.test/icon-128.png" {
		t.Fatalf("unexpected icon %q", app.Icon)
	}
	if len(app.Screenshots) != 1 || app.Screenshots[0].URLs[0] != "https://example.test/shot-large.png" {
		t.Fatalf("unexpected screenshots %#v", app.Screenshots)
	}
}

func TestFlathubAppRejectsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	_, err := (FlathubClient{BaseURL: server.URL}).App(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected HTTP failure")
	}
}

func TestFlathubSearchAllPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		var body struct {
			Query       string `json:"query"`
			HitsPerPage int    `json:"hits_per_page"`
			Page        int    `json:"page"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Query != "" || body.HitsPerPage != 1000 {
			t.Fatalf("unexpected search request %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		if body.Page == 1 {
			_, _ = w.Write([]byte(`{"hits":[{"id":"org.example.One","app_id":"org.example.One","name":"One","summary":"First","description":"<p>First app</p>","developer_name":"Dev","icon":"https://example.test/one.png","keywords":["one"],"main_categories":"Utility","sub_categories":["Calculator"],"verification_verified":true}],"page":1,"totalPages":2,"totalHits":2}`))
			return
		}
		_, _ = w.Write([]byte(`{"hits":[{"id":"org.example.Two","app_id":"org.example.Two","name":"Two","summary":"Second","description":"Second app","main_categories":["Game","Network"],"sub_categories":["Chat"]}],"page":2,"totalPages":2,"totalHits":2}`))
	}))
	defer server.Close()

	apps, total, err := (FlathubClient{BaseURL: server.URL}).SearchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(apps) != 2 {
		t.Fatalf("unexpected search result total=%d apps=%#v", total, apps)
	}
	if apps[0].Description != "First app" || !apps[0].Verified {
		t.Fatalf("unexpected first app %#v", apps[0])
	}
	if len(apps[0].SubCategories) != 1 || apps[0].SubCategories[0] != "Calculator" {
		t.Fatalf("unexpected first app subcategories %#v", apps[0].SubCategories)
	}
	if len(apps[1].Categories) != 2 || apps[1].Categories[0] != "Game" {
		t.Fatalf("unexpected categories %#v", apps[1].Categories)
	}
	if len(apps[1].SubCategories) != 1 || apps[1].SubCategories[0] != "Chat" {
		t.Fatalf("unexpected subcategories %#v", apps[1].SubCategories)
	}
}
