package web

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The mark lives here, and the whoctl binary carries a copy of it.
//
// # Why a copy exists at all
//
// The server renders a page, and it has to do that with no site checked out
// beside it and no route to the internet — which is the ordinary case for
// something holding infrastructure credentials. Referencing the published site
// would make the mark disappear exactly where the server is most likely to run.
//
// # Why this test exists
//
// Because two copies of one file drift, and nothing else would say so. `make
// logo` already has this problem with the PNGs, and the note about it says
// plainly that editing the SVG without re-exporting leaves them disagreeing
// with nothing to tell you. This is the telling, for the one copy that lives in
// another repository.
//
// It runs only when whoctl is checked out beside this one, which is the only
// moment the drift can matter: a lone clone of this repository cannot fix a
// file it does not have.
func TestTheMarkTheServerCarriesIsThisOne(t *testing.T) {
	const copyPath = "../../whoctl/internal/server/brand/logo.svg"

	theirs, err := os.ReadFile(copyPath)
	if os.IsNotExist(err) {
		t.Skip("whoctl is not checked out beside this repository")
	}
	if err != nil {
		t.Fatalf("reading %s: %v", copyPath, err)
	}
	ours, err := FS.ReadFile("assets/logo.svg")
	if err != nil {
		t.Fatalf("reading the mark: %v", err)
	}
	if !bytes.Equal(ours, theirs) {
		t.Errorf("%s is not this repository's web/assets/logo.svg.\n"+
			"The mark lives here; copy it over:\n  cp web/assets/logo.svg %s",
			filepath.Clean(copyPath), filepath.Clean(copyPath))
	}
}

// The icon travels the same way and for the same reason.
func TestTheIconTheServerCarriesIsThisOne(t *testing.T) {
	const copyPath = "../../whoctl/internal/server/brand/favicon.ico"

	theirs, err := os.ReadFile(copyPath)
	if os.IsNotExist(err) {
		t.Skip("whoctl is not checked out beside this repository")
	}
	if err != nil {
		t.Fatalf("reading %s: %v", copyPath, err)
	}
	ours, err := FS.ReadFile("assets/favicon.ico")
	if err != nil {
		t.Fatalf("reading the icon: %v", err)
	}
	if !bytes.Equal(ours, theirs) {
		t.Errorf("%s is not this repository's web/assets/favicon.ico", filepath.Clean(copyPath))
	}
}
