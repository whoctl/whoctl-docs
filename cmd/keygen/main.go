// Command keygen generates the ed25519 key pair the registry index is signed
// with.
//
// It is run once, by a person, and never by CI: a key CI can generate is a key
// CI can regenerate, and the whole point of the pair is that replacing it takes
// a release of whoctl. Neither half is written to a file here — printing them
// is the last time this program is involved.
package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "keygen:", err)
		os.Exit(1)
	}

	fmt.Printf(`A signing key for the whoctl registry. The two halves go to two places, in
the same change — either one alone stops installation.

  WHOCTL_SIGNING_KEY, a secret on whoctl/whoctl-docs:

    %s

  officialKeyHex, in whoctl/internal/install/manager.go:

    %s

The private half signs every release whoctl will ever trust as official. It
belongs in the secret and nowhere else: not in a file, not in a password
manager shared wider than the people who publish, and not in this terminal's
scrollback for longer than it takes to paste it.
`, hex.EncodeToString(priv.Seed()), hex.EncodeToString(pub))
}
