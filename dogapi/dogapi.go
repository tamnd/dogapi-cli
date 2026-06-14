// Package dogapi is the library behind the dogapi command line:
// the HTTP client, request shaping, and typed data models for
// the Dog CEO's Dog API (dog.ceo).
//
// No API key is required. The client paces requests and retries on transient
// errors so a busy session stays polite.
package dogapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Host is the site this client talks to.
const Host = "dog.ceo"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://dog.ceo",
		UserAgent: "dogapi-cli/0.1 (tamnd87@gmail.com)",
		Rate:      100 * time.Millisecond,
		Timeout:   10 * time.Second,
		Retries:   3,
	}
}

// Client talks to dog.ceo over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Breed is a single dog breed entry.
type Breed struct {
	Name      string `kit:"id" json:"name"`
	SubBreeds string `json:"sub_breeds"` // comma-joined list of sub-breed names, or "" if none
}

// Image is a single dog image entry.
type Image struct {
	URL   string `kit:"id" json:"url"`
	Breed string `json:"breed"` // extracted from URL path or set from flag
}

// wire types for JSON decoding

type breedsResponse struct {
	Message map[string][]string `json:"message"`
	Status  string              `json:"status"`
}

type imagesListResponse struct {
	Message []string `json:"message"`
	Status  string   `json:"status"`
}

type imageSingleResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// ListBreeds fetches all dog breeds sorted alphabetically.
func (c *Client) ListBreeds(ctx context.Context) ([]Breed, error) {
	u := c.cfg.BaseURL + "/api/breeds/list/all"
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp breedsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode breeds: %w", err)
	}
	names := make([]string, 0, len(resp.Message))
	for name := range resp.Message {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]Breed, 0, len(names))
	for _, name := range names {
		subs := resp.Message[name]
		subStr := ""
		if len(subs) > 0 {
			subStr = strings.Join(subs, ",")
		}
		items = append(items, Breed{Name: name, SubBreeds: subStr})
	}
	return items, nil
}

// Random fetches count random dog images. breed and subBreed are optional.
// count must be >= 1.
func (c *Client) Random(ctx context.Context, breed, subBreed string, count int) ([]Image, error) {
	if count < 1 {
		count = 1
	}
	var u string
	switch {
	case breed == "":
		if count == 1 {
			u = c.cfg.BaseURL + "/api/breeds/image/random"
		} else {
			u = fmt.Sprintf("%s/api/breeds/image/random/%d", c.cfg.BaseURL, count)
		}
	case subBreed == "":
		if count == 1 {
			u = fmt.Sprintf("%s/api/breed/%s/images/random", c.cfg.BaseURL, breed)
		} else {
			u = fmt.Sprintf("%s/api/breed/%s/images/random/%d", c.cfg.BaseURL, breed, count)
		}
	default:
		// sub-breed: count param not supported by the API for this endpoint
		u = fmt.Sprintf("%s/api/breed/%s/%s/images/random", c.cfg.BaseURL, breed, subBreed)
	}
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	return decodeImageResponse(body)
}

// ListImages fetches all images for a breed (and optional sub-breed), then
// applies the client-side limit.
func (c *Client) ListImages(ctx context.Context, breed, subBreed string, limit int) ([]Image, error) {
	var u string
	if subBreed == "" {
		u = fmt.Sprintf("%s/api/breed/%s/images", c.cfg.BaseURL, breed)
	} else {
		u = fmt.Sprintf("%s/api/breed/%s/%s/images", c.cfg.BaseURL, breed, subBreed)
	}
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	items, err := decodeImageResponse(body)
	if err != nil {
		return nil, err
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items, nil
}

// extractBreedFromURL extracts the breed name from a dog.ceo image URL.
// URL format: https://images.dog.ceo/breeds/{dir}/{filename}
// Sub-breed dirs look like "hound-afghan"; we return "hound/afghan".
func extractBreedFromURL(rawURL string) string {
	const marker = "/breeds/"
	idx := strings.Index(rawURL, marker)
	if idx < 0 {
		return ""
	}
	rest := rawURL[idx+len(marker):]
	end := strings.Index(rest, "/")
	if end >= 0 {
		rest = rest[:end]
	}
	// "hound-afghan" -> "hound/afghan"
	return strings.ReplaceAll(rest, "-", "/")
}

// decodeImageResponse handles both single-string and array message formats.
func decodeImageResponse(body []byte) ([]Image, error) {
	// try array first
	var arrResp imagesListResponse
	if err := json.Unmarshal(body, &arrResp); err == nil && arrResp.Message != nil {
		items := make([]Image, 0, len(arrResp.Message))
		for _, u := range arrResp.Message {
			items = append(items, Image{URL: u, Breed: extractBreedFromURL(u)})
		}
		return items, nil
	}
	// try single string
	var singleResp imageSingleResponse
	if err := json.Unmarshal(body, &singleResp); err == nil && singleResp.Message != "" {
		return []Image{{URL: singleResp.Message, Breed: extractBreedFromURL(singleResp.Message)}}, nil
	}
	return nil, fmt.Errorf("decode image response: unexpected format")
}

func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, u)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", u, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
