package registry

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"
)

// SigningKeyEnv holds the hex-encoded ed25519 private key the index is signed
// with.
//
// It is a secret and it never leaves the workflow that publishes the index. The
// public half is compiled into whoctl, which is what makes replacing the key a
// release of whoctl rather than an edit to a file the same server serves.
const SigningKeyEnv = "WHOCTL_SIGNING_KEY"

// KeySigner signs release checksums with an ed25519 key.
type KeySigner struct{ key ed25519.PrivateKey }

// NewSigner reads the key from its hex encoding. An empty string is not an
// error: it means this build publishes an unsigned index, which whoctl accepts
// for a namespace it has no key for.
func NewSigner(hexKey string) (*KeySigner, error) {
	hexKey = strings.TrimSpace(hexKey)
	if hexKey == "" {
		return nil, nil
	}
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("%s is not hex: %w", SigningKeyEnv, err)
	}
	switch len(raw) {
	case ed25519.PrivateKeySize:
		return &KeySigner{key: ed25519.PrivateKey(raw)}, nil
	case ed25519.SeedSize:
		// A 32-byte seed is what most tools print; expanding it here means the
		// key can be pasted in whichever form it was generated.
		return &KeySigner{key: ed25519.NewKeyFromSeed(raw)}, nil
	default:
		return nil, fmt.Errorf("%s is %d bytes, want %d or %d", SigningKeyEnv, len(raw), ed25519.SeedSize, ed25519.PrivateKeySize)
	}
}

// Sign signs the lowercase hex checksum, which is exactly the message whoctl
// verifies. The signature covers the checksum and the checksum covers the
// bytes, so one small signature stands for the whole archive.
func (s *KeySigner) Sign(sha256Hex string) (string, error) {
	if len(sha256Hex) != 64 {
		return "", fmt.Errorf("%q is not a sha256 checksum", sha256Hex)
	}
	sig := ed25519.Sign(s.key, []byte(strings.ToLower(sha256Hex)))
	return hex.EncodeToString(sig), nil
}

// PublicKeyHex is what goes into whoctl's officialKeyHex.
func (s *KeySigner) PublicKeyHex() string {
	return hex.EncodeToString(s.key.Public().(ed25519.PublicKey))
}
