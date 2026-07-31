// Command whoctl-docs builds the whoctl documentation site.
//
// It renders one site from the documentation of many providers. A provider
// publishes a bundle — its pages and its schema, no markup — with each release;
// providers.yaml says which ones the site covers, and everything about how they
// look is in this repository, in one copy.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/whoctl/whoctl-sdk-go/docs"

	"github.com/whoctl/whoctl-docs/internal/site"
)

func main() {
	catalogue := flag.String("providers", "providers.yaml", "the catalogue of providers the site covers")
	output := flag.String("o", "site", "directory to write the site into")
	version := flag.String("version", "dev", "shown in the site header")
	flag.Parse()

	if err := build(*catalogue, *output, *version); err != nil {
		fmt.Fprintln(os.Stderr, "whoctl-docs:", err)
		os.Exit(1)
	}
}

func build(cataloguePath, output, version string) error {
	catalogue, err := site.ReadCatalogue(cataloguePath)
	if err != nil {
		return err
	}
	bundles, err := catalogue.Fetch(context.Background())
	if err != nil {
		return err
	}

	files, err := site.Render(docs.SiteOf(bundles, docs.Options{Version: version}))
	if err != nil {
		return err
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
