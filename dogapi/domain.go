package dogapi

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes dogapi as a kit Domain driver.
//
// A multi-domain host (ant) enables it with a single blank import:
//
//	import _ "github.com/tamnd/dogapi-cli/dogapi"
//
// The same Domain also builds the standalone dogapi binary (see cli.NewApp).
func init() { kit.Register(Domain{}) }

// Domain is the dogapi driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "dogapi",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "dogapi",
			Short:  "Dog breeds and random images from dog.ceo.",
			Long: `dogapi fetches dog breeds and random dog images from the public Dog CEO API.
No login or API key required.`,
			Site: Host,
			Repo: "https://github.com/tamnd/dogapi-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// breeds: list all dog breeds sorted alphabetically
	kit.Handle(app, kit.OpMeta{
		Name:    "breeds",
		Group:   "read",
		List:    true,
		Summary: "List all dog breeds",
	}, breedsOp)

	// random: fetch random dog image(s), optionally filtered by breed
	kit.Handle(app, kit.OpMeta{
		Name:    "random",
		Group:   "read",
		List:    true,
		Summary: "Fetch random dog image(s)",
	}, randomOp)

	// images: list images for a specific breed
	kit.Handle(app, kit.OpMeta{
		Name:    "images",
		Group:   "read",
		List:    true,
		Summary: "List images for a breed",
		Args:    []kit.Arg{{Name: "breed", Help: "breed name (e.g. labrador)"}},
	}, imagesOp)
}

// newClient builds the client from host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type breedsInput struct {
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

type randomInput struct {
	Breed    string  `kit:"flag" help:"breed name (e.g. labrador)"`
	SubBreed string  `kit:"flag,name=sub-breed" help:"sub-breed name (e.g. french for bulldog/french)"`
	Count    int     `kit:"flag" help:"number of random images (1-50)" default:"1"`
	Client   *Client `kit:"inject"`
}

type imagesInput struct {
	Breed    string  `kit:"arg" help:"breed name (e.g. labrador)"`
	SubBreed string  `kit:"flag,name=sub-breed" help:"sub-breed name (e.g. french for bulldog)"`
	Limit    int     `kit:"flag,inherit" help:"max images (0 = all)" default:"20"`
	Client   *Client `kit:"inject"`
}

// --- handlers ---

func breedsOp(ctx context.Context, in breedsInput, emit func(Breed) error) error {
	items, err := in.Client.ListBreeds(ctx)
	if err != nil {
		return mapErr(err)
	}
	limit := in.Limit
	for i, item := range items {
		if limit > 0 && i >= limit {
			break
		}
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

func randomOp(ctx context.Context, in randomInput, emit func(Image) error) error {
	count := in.Count
	if count < 1 {
		count = 1
	}
	items, err := in.Client.Random(ctx, in.Breed, in.SubBreed, count)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

func imagesOp(ctx context.Context, in imagesInput, emit func(Image) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	items, err := in.Client.ListImages(ctx, in.Breed, in.SubBreed, limit)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify turns an input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("empty dogapi reference")
	}
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return "url", input, nil
	}
	return "breed", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "breed":
		return "https://dog.ceo/api/breed/" + id + "/images", nil
	case "url":
		return id, nil
	default:
		return "", errs.Usage("dogapi has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind.
func mapErr(err error) error {
	return err
}
