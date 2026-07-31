// Command whoctl-docs builds the whoctl documentation site.
//
// It renders one site from the documentation of many providers. A provider
// publishes a bundle — its pages and its schema, no markup — with each release;
// providers.yaml says which ones the site covers, and everything about how they
// look is in this repository, in one copy.
//
// The same run writes the registry index under registry/, which is what whoctl
// installs providers from. The two are published together because they answer
// the same question from the same catalogue — what providers are there — and
// splitting them across two deployments is how a site ends up documenting a
// version nobody can install.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/whoctl/whoctl-sdk-go/docs"

	"github.com/whoctl/whoctl-docs/internal/registry"
	"github.com/whoctl/whoctl-docs/internal/site"
)

func main() {
	catalogue := flag.String("providers", "providers.yaml", "the catalogue of providers the site covers")
	output := flag.String("o", "site", "directory to write the site into")
	version := flag.String("version", "dev", "shown in the site header")
	index := flag.Bool("registry", true, "also write the registry index whoctl installs from")
	flag.Parse()

	if err := build(*catalogue, *output, *version, *index); err != nil {
		fmt.Fprintln(os.Stderr, "whoctl-docs:", err)
		os.Exit(1)
	}
}

func build(cataloguePath, output, version string, index bool) error {
	ctx := context.Background()
	catalogue, err := site.ReadCatalogue(cataloguePath)
	if err != nil {
		return err
	}
	bundles, err := catalogue.Fetch(ctx)
	if err != nil {
		return err
	}

	// The SDK's default title is "whoctl registry", which was fine when this
	// was the only thing published here. It is not any more: the registry is
	// now the index next door, and two things called that is one too many.
	repos := map[string]string{}
	for _, s := range catalogue.Providers {
		repos[s.Name] = s.Repository
	}

	files, err := site.Render(docs.SiteOf(bundles, docs.Options{Title: "whoctl docs", Version: version}), repos)
	if err != nil {
		return err
	}

	if index {
		signer, err := registry.NewSigner(os.Getenv(registry.SigningKeyEnv))
		if err != nil {
			return err
		}
		// A nil *KeySigner is not a nil Signer, and calling Sign on it would
		// panic rather than publish an unsigned index.
		var sign registry.Signer
		if signer != nil {
			sign = signer
		}
		forge := registry.NewGitHub(os.Getenv("GITHUB_TOKEN"))
		forge.Log = logf
		entries, err := registry.Build(ctx, catalogue.Registry(), forge, sign, logf)
		if err != nil {
			return err
		}
		for name, content := range entries {
			files[name] = content
		}
	}
	for name, content := range files {
		path := filepath.Join(output, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("wrote %d files into %s, covering %d providers\n", len(files), output, len(bundles))
	return nil
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
