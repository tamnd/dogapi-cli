package dogapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/dogapi-cli/dogapi"
)

const fakeBreedsJSON = `{"message":{"bulldog":["boston","english","french"],"husky":[],"labrador":[]},"status":"success"}`

const fakeImagesArrayJSON = `{"message":["https://images.dog.ceo/breeds/husky/img1.jpg","https://images.dog.ceo/breeds/husky/img2.jpg"],"status":"success"}`

const fakeRandomSingleJSON = `{"message":"https://images.dog.ceo/breeds/labrador/n02099712_3340.jpg","status":"success"}`

func newTestClient(ts *httptest.Server) *dogapi.Client {
	cfg := dogapi.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return dogapi.NewClient(cfg)
}

func TestListBreedsSendsUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, fakeBreedsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.ListBreeds(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotUA == "" {
		t.Error("User-Agent not sent")
	}
}

func TestListBreedsParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeBreedsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.ListBreeds(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	// sorted alphabetically: bulldog=0, husky=1, labrador=2
	if items[0].Name != "bulldog" {
		t.Errorf("items[0].Name = %q, want bulldog", items[0].Name)
	}
	if items[0].SubBreeds != "boston,english,french" {
		t.Errorf("items[0].SubBreeds = %q, want boston,english,french", items[0].SubBreeds)
	}
	if items[1].Name != "husky" {
		t.Errorf("items[1].Name = %q, want husky", items[1].Name)
	}
	if items[1].SubBreeds != "" {
		t.Errorf("items[1].SubBreeds = %q, want empty string", items[1].SubBreeds)
	}
	if items[2].Name != "labrador" {
		t.Errorf("items[2].Name = %q, want labrador", items[2].Name)
	}
}

func TestListBreedsRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, fakeBreedsJSON)
	}))
	defer ts.Close()

	cfg := dogapi.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := dogapi.NewClient(cfg)

	_, err := c.ListBreeds(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestRandomSingleNoBreed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/breeds/image/random" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, fakeRandomSingleJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Random(context.Background(), "", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("len = %d, want 1", len(items))
	}
	if items[0].URL != "https://images.dog.ceo/breeds/labrador/n02099712_3340.jpg" {
		t.Errorf("URL = %q", items[0].URL)
	}
	if items[0].Breed != "labrador" {
		t.Errorf("Breed = %q, want labrador", items[0].Breed)
	}
}

func TestRandomMultipleNoBreed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/breeds/image/random/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, fakeImagesArrayJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Random(context.Background(), "", "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2 (fake response)", len(items))
	}
}

func TestRandomWithBreed(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = fmt.Fprint(w, fakeImagesArrayJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Random(context.Background(), "husky", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotPath, "/breed/husky/") {
		t.Errorf("path = %q, want /breed/husky/...", gotPath)
	}
}

func TestRandomWithSubBreed(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = fmt.Fprint(w, fakeRandomSingleJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Random(context.Background(), "hound", "afghan", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotPath, "/breed/hound/afghan/") {
		t.Errorf("path = %q, want /breed/hound/afghan/...", gotPath)
	}
}

func TestListImagesParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeImagesArrayJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.ListImages(context.Background(), "husky", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].URL != "https://images.dog.ceo/breeds/husky/img1.jpg" {
		t.Errorf("items[0].URL = %q", items[0].URL)
	}
	if items[0].Breed != "husky" {
		t.Errorf("items[0].Breed = %q, want husky", items[0].Breed)
	}
}

func TestListImagesLimitClientSide(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeImagesArrayJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.ListImages(context.Background(), "husky", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("len = %d, want 1 (limit applied client-side)", len(items))
	}
}

func TestListImagesSubBreedPath(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = fmt.Fprint(w, fakeImagesArrayJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.ListImages(context.Background(), "hound", "afghan", 0)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/breed/hound/afghan/images" {
		t.Errorf("path = %q, want /api/breed/hound/afghan/images", gotPath)
	}
}
