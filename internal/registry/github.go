package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// maxPages bounds pagination. A hundred releases per page and five pages is far
// past anything real; it exists so a forge that keeps answering "here is one
// more page" cannot turn a site build into an infinite loop.
const maxPages = 5

// GitHub reads published releases from the GitHub API.
type GitHub struct {
	HTTP *http.Client
	// Token authenticates the API calls. Without one GitHub allows sixty
	// requests an hour per address, which a site build shares with everything
	// else on the runner. Actions provides one for free.
	Token string
	// APIBase overrides the API root. GitHub Actions sets GITHUB_API_URL on
	// every runner — to api.github.com, or to the host of a GitHub Enterprise
	// installation — so honouring it costs nothing and is also what lets the
	// index be built against a stand-in server with no network at all.
	APIBase string
	Log     func(string, ...any)
}

// NewGitHub builds a client with a bounded timeout.
func NewGitHub(token string) *GitHub {
	return &GitHub{
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		Token:   token,
		APIBase: os.Getenv("GITHUB_API_URL"),
	}
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Artifacts lists every platform build of every published release.
//
// A draft is not published and is skipped. A GitHub prerelease is not: whether
// a version is a prerelease is decided by the tag itself, because that is what
// whoctl's constraints read, and a release flagged one way with a tag saying
// the other would install differently than it displays.
func (g *GitHub) Artifacts(ctx context.Context, repository, provider string) ([]Artifact, error) {
	var out []Artifact
	prefix := "whoctl-provider-" + provider + "-"

	for page := 1; page <= maxPages; page++ {
		releases, err := g.page(ctx, repository, page)
		if err != nil {
			return nil, err
		}
		for _, r := range releases {
			if r.Draft {
				continue
			}
			version, ok := indexVersion(r.TagName)
			if !ok {
				g.log("registry: %s: tag %q is not a version whoctl can read, skipping", repository, r.TagName)
				continue
			}
			// The checksums are separate assets, so the archives cannot be
			// turned into index entries until every asset of this release has
			// been seen.
			sums := map[string]string{}
			for _, a := range r.Assets {
				if !strings.HasSuffix(a.Name, ".tar.gz.sha256") {
					continue
				}
				sum, err := g.checksum(ctx, a.URL)
				if err != nil {
					return nil, fmt.Errorf("%s %s: %w", repository, a.Name, err)
				}
				sums[strings.TrimSuffix(a.Name, ".sha256")] = sum
			}
			for _, a := range r.Assets {
				if !strings.HasPrefix(a.Name, prefix) || !strings.HasSuffix(a.Name, ".tar.gz") {
					continue
				}
				sum, ok := sums[a.Name]
				if !ok {
					// whoctl refuses to install what it cannot verify, so an
					// archive with no checksum published beside it is left out
					// rather than written into the index as unusable.
					g.log("registry: %s %s: no checksum published beside %s, leaving it out", repository, r.TagName, a.Name)
					continue
				}
				out = append(out, Artifact{
					Version:  version,
					Platform: strings.TrimSuffix(strings.TrimPrefix(a.Name, prefix), ".tar.gz"),
					URL:      a.URL,
					SHA256:   sum,
				})
			}
		}
		if len(releases) < 100 {
			return out, nil
		}
		if page == maxPages {
			g.log("registry: %s has more than %d releases; the oldest are not in the index", repository, maxPages*100)
		}
	}
	return out, nil
}

func (g *GitHub) page(ctx context.Context, repository string, page int) ([]ghRelease, error) {
	base := g.APIBase
	if base == "" {
		base = "https://api.github.com"
	}
	url := base + "/repos/" + repository + "/releases?per_page=100&page=" + strconv.Itoa(page)

	body, err := g.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var releases []ghRelease
	if err := json.NewDecoder(io.LimitReader(body, 8<<20)).Decode(&releases); err != nil {
		return nil, fmt.Errorf("reading the releases of %s: %w", repository, err)
	}
	return releases, nil
}

// checksum reads a published .sha256 file, which sha256sum writes as
// "<hash>  <name>".
func (g *GitHub) checksum(ctx context.Context, url string) (string, error) {
	body, err := g.get(ctx, url)
	if err != nil {
		return "", err
	}
	defer body.Close()

	raw, err := io.ReadAll(io.LimitReader(body, 4<<10))
	if err != nil {
		return "", err
	}
	hash, _, _ := strings.Cut(strings.TrimSpace(string(raw)), " ")
	if len(hash) != 64 {
		return "", fmt.Errorf("%q is not a sha256 checksum", strings.TrimSpace(string(raw)))
	}
	return strings.ToLower(hash), nil
}

func (g *GitHub) get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	client := g.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return nil, fmt.Errorf("fetching %s: rate limited by GitHub; set GITHUB_TOKEN", url)
		}
		return nil, fmt.Errorf("fetching %s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

func (g *GitHub) log(format string, args ...any) {
	if g.Log != nil {
		g.Log(format, args...)
	}
}

// indexVersion turns a release tag into the version the index spells, or
// reports that whoctl could not read it.
//
// This is deliberately the same shape whoctl's ParseVersion accepts. A tag it
// cannot read would become a versions.json entry the client silently drops,
// and a version that is in the index but never installable is worse than one
// that was never listed.
func indexVersion(tag string) (string, bool) {
	raw := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	core, pre, hasPre := strings.Cut(raw, "-")
	if hasPre && pre == "" {
		return "", false
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return "", false
	}
	for _, part := range parts {
		if n, err := strconv.Atoi(part); err != nil || n < 0 {
			return "", false
		}
	}
	return raw, true
}
