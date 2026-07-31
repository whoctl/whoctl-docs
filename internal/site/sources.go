package site

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/whoctl/whoctl-sdk-go/docs"
	"gopkg.in/yaml.v3"

	"github.com/whoctl/whoctl-docs/internal/registry"
)

// Catalogue is providers.yaml: which providers the site covers, and where each
// one's documentation bundle is.
//
// It is the site's editorial control and the only place it lives. Nothing in a
// provider's own repository can add it to the site or take it off, which is
// what makes a central build a decision rather than an accident.
type Catalogue struct {
	Providers []Source `yaml:"providers"`
}

// Source is one provider at one version.
type Source struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	// Bundle is a URL or a path. A path is how the workspace builds the site
	// from providers checked out beside it, before anything is published.
	Bundle string `yaml:"bundle"`
	// Repository is "owner/repo", and it is what the registry index is derived
	// from: the published releases of that repository are the versions whoctl
	// can install. A provider with none is documented and not installable,
	// which is exactly a workspace build against a sibling checkout.
	Repository string `yaml:"repository"`
	// Namespace addresses the provider, as in whoctl/linux. Empty means the
	// official one, which is the only namespace a bare name resolves in.
	Namespace string `yaml:"namespace"`
}

// OfficialNamespace is where a provider lives when the catalogue does not say.
const OfficialNamespace = "whoctl"

// Registry describes the providers as the index generator needs them.
func (c *Catalogue) Registry() []registry.Source {
	out := make([]registry.Source, 0, len(c.Providers))
	for _, s := range c.Providers {
		namespace := s.Namespace
		if namespace == "" {
			namespace = OfficialNamespace
		}
		out = append(out, registry.Source{Namespace: namespace, Name: s.Name, Repository: s.Repository})
	}
	return out
}

// ReadCatalogue parses providers.yaml.
func ReadCatalogue(path string) (*Catalogue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Catalogue
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(c.Providers) == 0 {
		return nil, fmt.Errorf("%s lists no providers, so the site would have nothing in it", path)
	}
	for i, s := range c.Providers {
		if s.Name == "" || s.Bundle == "" {
			return nil, fmt.Errorf("%s: entry %d needs a name and a bundle", path, i+1)
		}
	}
	return &c, nil
}

// Fetch reads every bundle the catalogue names.
//
// One unreadable bundle fails the build rather than producing a site quietly
// missing a provider: a page that is not there is much harder to notice than a
// build that stopped.
func (c *Catalogue) Fetch(ctx context.Context) ([]*docs.Bundle, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	out := make([]*docs.Bundle, 0, len(c.Providers))

	for _, source := range c.Providers {
		b, err := fetchBundle(ctx, client, source)
		if err != nil {
			return nil, fmt.Errorf("provider %s: %w", source.Name, err)
		}
		if b.Provider.Name != source.Name {
			return nil, fmt.Errorf("%s claims to be the %q provider, but the catalogue calls it %q",
				source.Bundle, b.Provider.Name, source.Name)
		}
		if source.Version != "" {
			// The catalogue is what decides which version a page is published
			// under, so it wins over whatever the bundle says it was built at.
			b.Version = source.Version
		}
		out = append(out, b)
	}
	return out, nil
}

func fetchBundle(ctx context.Context, client *http.Client, source Source) (*docs.Bundle, error) {
	if !strings.HasPrefix(source.Bundle, "http://") && !strings.HasPrefix(source.Bundle, "https://") {
		f, err := os.Open(source.Bundle)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return docs.ReadBundle(f)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.Bundle, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: %s", source.Bundle, resp.Status)
	}
	return docs.ReadBundle(io.LimitReader(resp.Body, 16<<20))
}
