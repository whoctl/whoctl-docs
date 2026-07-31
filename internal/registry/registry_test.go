package registry

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

const (
	sumA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sumB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// TestIndexShape pins the wire format.
//
// The reader is in another repository — whoctl/internal/install/index.go — so
// nothing makes these two agree except this test and the comment above it.
// Renaming a field here and nowhere else breaks every installation until whoctl
// is rebuilt, and it breaks it at the far end, where the message will be about
// a missing checksum rather than about a rename.
func TestIndexShape(t *testing.T) {
	blob, err := marshal(Release{
		Version: "0.1.0", Platform: "linux_amd64",
		URL: "https://example.invalid/p.tar.gz", SHA256: sumA, Signature: "cafe",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"version": "0.1.0", "platform": "linux_amd64",
		"url": "https://example.invalid/p.tar.gz", "sha256": sumA, "signature": "cafe",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("release[%q] = %v, want %v", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("release has %d fields, want %d: %v", len(got), len(want), got)
	}

	// An unsigned release omits the field rather than sending an empty string,
	// which is what lets whoctl tell "not signed" from "signed with nothing".
	blob, err = marshal(Release{Version: "0.1.0", Platform: "linux_amd64", URL: "u", SHA256: sumA})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(blob), "signature") {
		t.Errorf("an unsigned release carries a signature field: %s", blob)
	}

	blob, err = marshal(Versions{Versions: []string{"0.1.0"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(strings.Fields(string(blob)), " "); got != `{ "versions": [ "0.1.0" ] }` {
		t.Errorf("versions.json is %s", got)
	}
}

type fakeForge map[string][]Artifact

func (f fakeForge) Artifacts(_ context.Context, repository, _ string) ([]Artifact, error) {
	return f[repository], nil
}

func TestBuildWritesTheWholeIndex(t *testing.T) {
	forge := fakeForge{"whoctl/whoctl-provider-linux": {
		{Version: "0.2.0", Platform: "linux_amd64", URL: "u1", SHA256: sumA},
		{Version: "0.1.0", Platform: "linux_amd64", URL: "u2", SHA256: sumB},
		{Version: "0.1.0", Platform: "linux_arm64", URL: "u3", SHA256: sumB},
	}}
	files, err := Build(context.Background(),
		[]Source{{Namespace: "whoctl", Name: "linux", Repository: "whoctl/whoctl-provider-linux"}},
		forge, nil, func(string, ...any) {})
	if err != nil {
		t.Fatal(err)
	}

	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	want := []string{
		"registry/whoctl/linux/0.1.0/linux_amd64.json",
		"registry/whoctl/linux/0.1.0/linux_arm64.json",
		"registry/whoctl/linux/0.2.0/linux_amd64.json",
		"registry/whoctl/linux/versions.json",
	}
	if strings.Join(paths, " ") != strings.Join(want, " ") {
		t.Fatalf("wrote\n  %v\nwant\n  %v", paths, want)
	}

	var versions Versions
	if err := json.Unmarshal(files["registry/whoctl/linux/versions.json"], &versions); err != nil {
		t.Fatal(err)
	}
	if strings.Join(versions.Versions, " ") != "0.1.0 0.2.0" {
		t.Errorf("versions.json lists %v", versions.Versions)
	}
}

// A provider with no repository is a workspace build against a sibling
// checkout. It has published nothing, and the index must say nothing about it
// rather than claim it exists with no versions.
func TestBuildSkipsWhatIsNotPublished(t *testing.T) {
	for _, source := range []Source{
		{Namespace: "whoctl", Name: "linux"},
		{Namespace: "whoctl", Name: "steam", Repository: "whoctl/whoctl-provider-steam"},
	} {
		files, err := Build(context.Background(), []Source{source}, fakeForge{}, nil, func(string, ...any) {})
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 0 {
			t.Errorf("%s/%s wrote %v", source.Namespace, source.Name, files)
		}
	}
}

// The signature has to be verifiable by exactly what whoctl does with it, which
// is ed25519.Verify over the lowercase hex checksum — not over the archive, and
// not over the decoded bytes of the hash.
func TestSignedIndexVerifiesTheWayWhoctlVerifies(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewSigner(hex.EncodeToString(priv))
	if err != nil {
		t.Fatal(err)
	}
	if signer.PublicKeyHex() != hex.EncodeToString(pub) {
		t.Fatal("the signer reports a public key that is not its own")
	}

	files, err := Build(context.Background(),
		[]Source{{Namespace: "whoctl", Name: "linux", Repository: "r"}},
		fakeForge{"r": {{Version: "0.1.0", Platform: "linux_amd64", URL: "u", SHA256: strings.ToUpper(sumA)}}},
		signer, func(string, ...any) {})
	if err != nil {
		t.Fatal(err)
	}

	var release Release
	if err := json.Unmarshal(files["registry/whoctl/linux/0.1.0/linux_amd64.json"], &release); err != nil {
		t.Fatal(err)
	}
	sig, err := hex.DecodeString(release.Signature)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(pub, []byte(strings.ToLower(release.SHA256)), sig) {
		t.Error("whoctl would reject this signature")
	}
}

// A seed and a full private key are both things a key generator prints.
func TestSignerTakesEitherEncoding(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	full, err := NewSigner(hex.EncodeToString(priv))
	if err != nil {
		t.Fatal(err)
	}
	seed, err := NewSigner(hex.EncodeToString(priv.Seed()))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := full.Sign(sumA)
	b, _ := seed.Sign(sumA)
	if a != b {
		t.Error("the same key in two encodings signs differently")
	}

	if s, err := NewSigner("  "); err != nil || s != nil {
		t.Errorf("an absent key is not an error: %v, %v", s, err)
	}
	if _, err := NewSigner("abcd"); err == nil {
		t.Error("a key of the wrong length was accepted")
	}
}

func TestIndexVersion(t *testing.T) {
	cases := map[string]string{
		"v0.1.0":      "0.1.0",
		"0.1.0":       "0.1.0",
		"v1.2.3-rc.1": "1.2.3-rc.1",
		"v1.2":        "",
		"v1.2.3.4":    "",
		"latest":      "",
		"v1.2.x":      "",
		"v1.2.3-":     "",
	}
	for tag, want := range cases {
		got, ok := indexVersion(tag)
		if want == "" && ok {
			t.Errorf("%q was read as %q, want rejected", tag, got)
		}
		if want != "" && got != want {
			t.Errorf("%q -> %q, want %q", tag, got, want)
		}
	}
}

func TestArtifactsReadsAPublishedRelease(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".sha256"):
			w.Write([]byte(sumA + "  whoctl-provider-linux-linux_amd64.tar.gz\n"))
		case strings.Contains(r.URL.Path, "/releases"):
			if r.URL.Query().Get("page") != "1" {
				w.Write([]byte(`[]`))
				return
			}
			json.NewEncoder(w).Encode([]map[string]any{
				{"tag_name": "v0.1.0", "assets": []map[string]string{
					{"name": "whoctl-provider-linux-linux_amd64.tar.gz", "browser_download_url": server.URL + "/a.tar.gz"},
					{"name": "whoctl-provider-linux-linux_amd64.tar.gz.sha256", "browser_download_url": server.URL + "/a.tar.gz.sha256"},
					// Another platform, published with no checksum beside it.
					{"name": "whoctl-provider-linux-linux_arm64.tar.gz", "browser_download_url": server.URL + "/b.tar.gz"},
					// Not this provider's artifact at all.
					{"name": "docs-bundle.json", "browser_download_url": server.URL + "/docs-bundle.json"},
				}},
				{"tag_name": "v0.9.0", "draft": true, "assets": []map[string]string{
					{"name": "whoctl-provider-linux-linux_amd64.tar.gz", "browser_download_url": server.URL + "/draft.tar.gz"},
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	gh := &GitHub{APIBase: server.URL, Log: func(string, ...any) {}}
	got, err := gh.Artifacts(context.Background(), "whoctl/whoctl-provider-linux", "linux")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("read %d artifacts, want 1: %+v", len(got), got)
	}
	want := Artifact{Version: "0.1.0", Platform: "linux_amd64", URL: server.URL + "/a.tar.gz", SHA256: sumA}
	if got[0] != want {
		t.Errorf("read %+v, want %+v", got[0], want)
	}
}
