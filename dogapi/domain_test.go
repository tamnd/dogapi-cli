package dogapi

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and internal helpers.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "dogapi" {
		t.Errorf("Scheme = %q, want dogapi", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "dogapi" {
		t.Errorf("Identity.Binary = %q, want dogapi", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify empty string should return error")
	}

	typ, id, err := Domain{}.Classify("husky")
	if err != nil {
		t.Errorf("Classify: unexpected error: %v", err)
	}
	if typ != "breed" {
		t.Errorf("Classify type = %q, want breed", typ)
	}
	if id != "husky" {
		t.Errorf("Classify id = %q, want husky", id)
	}
}

func TestClassifyURL(t *testing.T) {
	rawURL := "https://images.dog.ceo/breeds/labrador/n02099712_3340.jpg"
	typ, id, err := Domain{}.Classify(rawURL)
	if err != nil {
		t.Fatalf("Classify URL: unexpected error: %v", err)
	}
	if typ != "url" {
		t.Errorf("Classify URL type = %q, want url", typ)
	}
	if id != rawURL {
		t.Errorf("Classify URL id = %q", id)
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("breed", "labrador")
	if err != nil {
		t.Fatalf("Locate: unexpected error: %v", err)
	}
	want := "https://dog.ceo/api/breed/labrador/images"
	if got != want {
		t.Errorf("Locate = %q, want %q", got, want)
	}

	_, err = Domain{}.Locate("unknown", "x")
	if err == nil {
		t.Error("Locate with unknown type should return error")
	}
}

func TestLocateURL(t *testing.T) {
	rawURL := "https://images.dog.ceo/breeds/labrador/n02099712_3340.jpg"
	got, err := Domain{}.Locate("url", rawURL)
	if err != nil {
		t.Fatalf("Locate url: unexpected error: %v", err)
	}
	if got != rawURL {
		t.Errorf("Locate url = %q, want %q", got, rawURL)
	}
}

func TestExtractBreedFromURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{
			url:  "https://images.dog.ceo/breeds/labrador/n02099712_3340.jpg",
			want: "labrador",
		},
		{
			url:  "https://images.dog.ceo/breeds/hound-afghan/img1.jpg",
			want: "hound/afghan",
		},
		{
			url:  "https://images.dog.ceo/breeds/bulldog-french/img1.jpg",
			want: "bulldog/french",
		},
		{
			url:  "https://images.dog.ceo/breeds/husky/img1.jpg",
			want: "husky",
		},
		{
			url:  "not-a-url",
			want: "",
		},
	}
	for _, tc := range cases {
		got := extractBreedFromURL(tc.url)
		if got != tc.want {
			t.Errorf("extractBreedFromURL(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestDecodeImageResponseSingle(t *testing.T) {
	body := []byte(`{"message":"https://images.dog.ceo/breeds/labrador/n02099712_3340.jpg","status":"success"}`)
	items, err := decodeImageResponse(body)
	if err != nil {
		t.Fatalf("decodeImageResponse: %v", err)
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

func TestDecodeImageResponseArray(t *testing.T) {
	body := []byte(`{"message":["https://images.dog.ceo/breeds/husky/img1.jpg","https://images.dog.ceo/breeds/husky/img2.jpg"],"status":"success"}`)
	items, err := decodeImageResponse(body)
	if err != nil {
		t.Fatalf("decodeImageResponse: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if items[0].Breed != "husky" {
		t.Errorf("Breed = %q, want husky", items[0].Breed)
	}
}

func TestDecodeImageResponseInvalid(t *testing.T) {
	body := []byte(`{"message":{},"status":"success"}`)
	_, err := decodeImageResponse(body)
	if err == nil {
		t.Error("expected error for invalid format, got nil")
	}
}
