// Package registry writes the static index whoctl installs providers from.
//
// A provider release publishes an archive per platform and a checksum beside
// each one. That is enough to download a provider and nothing else: it says
// nothing about which versions exist, and a client cannot discover them without
// asking GitHub, which rate limits and would tie whoctl to one forge. The index
// is the answer — plain JSON over HTTPS, laid out so a machine installing one
// provider fetches two small files:
//
//	<namespace>/<name>/versions.json
//	<namespace>/<name>/<version>/<os>_<arch>.json
//
// # The reader is whoctl/internal/install/index.go
//
// The two ends of this contract are in different repositories, so the shapes
// below are written out again rather than imported. Changing a field name here
// silently breaks installation everywhere until whoctl is rebuilt to match;
// TestIndexShape pins the wire names so the compiler is not the only thing
// standing between a rename and that. If this format ever needs to change,
// it belongs in the SDK first, where both sides can import one definition.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

// Versions is what versions.json holds: every version a provider has published,
// in no particular order. The client sorts, so a hand-edited index cannot be
// subtly wrong.
type Versions struct {
	Versions []string `json:"versions"`
}

// Release is one platform's metadata: where the artifact is, what it should
// hash to, and who says so.
type Release struct {
	Version  string `json:"version"`
	Platform string `json:"platform"`
	URL      string `json:"url"`
	// SHA256 is the hash of the archive, hex encoded. whoctl refuses to install
	// what it cannot verify, so a release with no checksum is left out of the
	// index entirely rather than published as unusable.
	SHA256 string `json:"sha256"`
	// Signature is that hash signed with the publisher's key, hex encoded. A
	// checksum only says the download was not corrupted in transit — it is
	// published by whoever published the artifact, so it proves nothing about
	// who that was. Only the signature does.
	Signature string `json:"signature,omitempty"`
}

// Source is one provider to index: which repository publishes it, and under
// what name whoctl addresses it.
type Source struct {
	Namespace string
	Name      string
	// Repository is "owner/repo" on GitHub. Empty means this provider is not
	// published yet — a workspace build against sibling checkouts — and it is
	// skipped rather than failing the build.
	Repository string
}

// Signer signs a release. It is optional: an unsigned index is what an
// unconfigured namespace produces, and whoctl treats that as a policy question
// rather than a failure.
type Signer interface {
	// Sign returns the hex-encoded signature over the lowercase hex checksum.
	// Signing the hash rather than the archive keeps verification cheap and
	// lets the signature live in the index next to what it signs.
	Sign(sha256Hex string) (string, error)
}

// Releases is what a forge tells us about a provider's published releases.
// GitHub is the only implementation; the interface exists so the builder can be
// tested without one.
type Releases interface {
	// Artifacts lists every published artifact of one repository. Order does
	// not matter.
	Artifacts(ctx context.Context, repository, provider string) ([]Artifact, error)
}

// Artifact is one platform's build of one version.
type Artifact struct {
	// Version is the release version without a leading v: tags carry it, paths
	// and JSON do not.
	Version  string
	Platform string
	URL      string
	SHA256   string
}

// Build renders the whole index as files to write, keyed by path relative to
// the site root.
//
// The index is derived, never accumulated: every build asks what is published
// right now and writes the answer whole. A version that was yanked disappears
// from versions.json on the next build, which a committed index would only do
// if somebody remembered to edit it.
func Build(ctx context.Context, sources []Source, forge Releases, signer Signer, log func(string, ...any)) (map[string][]byte, error) {
	out := map[string][]byte{}

	for _, s := range sources {
		if s.Repository == "" {
			log("registry: %s/%s has no repository, so it is not in the index", s.Namespace, s.Name)
			continue
		}
		artifacts, err := forge.Artifacts(ctx, s.Repository, s.Name)
		if err != nil {
			return nil, fmt.Errorf("provider %s/%s: %w", s.Namespace, s.Name, err)
		}

		versions := map[string]bool{}
		for _, a := range artifacts {
			r := Release{Version: a.Version, Platform: a.Platform, URL: a.URL, SHA256: a.SHA256}
			if signer != nil {
				sig, err := signer.Sign(a.SHA256)
				if err != nil {
					return nil, fmt.Errorf("signing %s/%s %s %s: %w", s.Namespace, s.Name, a.Version, a.Platform, err)
				}
				r.Signature = sig
			}
			blob, err := marshal(r)
			if err != nil {
				return nil, err
			}
			out[path.Join("registry", s.Namespace, s.Name, a.Version, a.Platform+".json")] = blob
			versions[a.Version] = true
		}

		if len(versions) == 0 {
			// Publishing an empty versions.json would turn "this provider has
			// released nothing yet" into "this provider does not exist", and
			// the two lead somebody to different next steps.
			log("registry: %s/%s has published nothing whoctl can install", s.Namespace, s.Name)
			continue
		}

		list := make([]string, 0, len(versions))
		for v := range versions {
			list = append(list, v)
		}
		sort.Strings(list)
		blob, err := marshal(Versions{Versions: list})
		if err != nil {
			return nil, err
		}
		out[path.Join("registry", s.Namespace, s.Name, "versions.json")] = blob
		log("registry: %s/%s has %d version(s), %s", s.Namespace, s.Name, len(list), strings.Join(list, " "))
	}
	return out, nil
}

func marshal(v any) ([]byte, error) {
	blob, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(blob, '\n'), nil
}
