package site

import (
	"strings"
	"testing"
)

func TestHighlightYAML(t *testing.T) {
	got := highlight("yaml", `apiVersion: linux.whoctl.io/v1alpha1
spec:
  uid: 4200
  locked: true
  comment: "Alice Liddell"
  groups:
    - wheel
# a comment`)

	for _, want := range []string{
		`<span class="tok-key">apiVersion</span>`,
		// A colon inside a scalar does not make the rest of it a value.
		`linux.whoctl.io/v1alpha1`,
		`<span class="tok-num">4200</span>`,
		`<span class="tok-lit">true</span>`,
		`<span class="tok-str">&quot;Alice Liddell&quot;</span>`,
		`<span class="tok-punct">-</span> wheel`,
		`<span class="tok-cmt"># a comment</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
	if strings.Contains(got, `tok-key">linux`) {
		t.Errorf("a colon inside a value must not start a key:\n%s", got)
	}
}

func TestHighlightConsole(t *testing.T) {
	got := highlight("console", `$ whoctl get user alice -o yaml
NAME     UID
alice    4200
whoctl-lab:~# whoctl delete user alice`)

	for _, want := range []string{
		`<span class="tok-prompt">$</span>`,
		`<span class="tok-cmd">whoctl</span>`,
		`<span class="tok-flag">-o</span>`,
		`<span class="tok-prompt">whoctl-lab:~#</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
	// Output lines carry no markup at all, so a table stays readable.
	for line := range strings.SplitSeq(got, "\n") {
		if strings.HasPrefix(line, "NAME") && strings.Contains(line, "<span") {
			t.Errorf("output line was highlighted: %s", line)
		}
	}
}

func TestHighlightShell(t *testing.T) {
	got := highlight("sh", `# build first
make run ARGS="get users -o wide" | grep alice`)

	for _, want := range []string{
		`<span class="tok-cmt"># build first</span>`,
		`<span class="tok-cmd">make</span>`,
		`<span class="tok-str">&quot;get users -o wide&quot;</span>`,
		`<span class="tok-punct">|</span>`,
		// The word after a pipe starts a new command.
		`<span class="tok-cmd">grep</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
}

func TestHighlightUnknownLanguageIsPlainEscapedText(t *testing.T) {
	const src = "NAME  <not html> & more\n$ not a command"
	if got := highlight("", src); got != escapeHTML(src) {
		t.Errorf("an untagged block must be escaped and nothing else:\n%s", got)
	}
}

func TestHighlightEscapesInsideTokens(t *testing.T) {
	got := highlight("sh", `echo "<b>" && x`)
	if strings.Contains(got, "<b>") {
		t.Errorf("markup leaked out of a string token: %s", got)
	}
	if !strings.Contains(got, "&lt;b&gt;") {
		t.Errorf("string token was not escaped: %s", got)
	}
}
