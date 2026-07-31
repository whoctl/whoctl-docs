package site

import (
	"strings"
	"testing"
)

func TestRenderMarkdownBlocks(t *testing.T) {
	src := `## Getting started

Text with ` + "`code`" + `, **bold**, *slanted* and a [link](user.html).

` + "```yaml" + `
kind: User
` + "```" + `

| Field | Type |
| --- | ---: |
| ` + "`uid`" + ` | integer |

- first
- second
  - nested
- third

> a quoted line

---
`

	html, headings := renderMarkdown(src)

	for _, want := range []string{
		`<h2 id="getting-started">Getting started</h2>`,
		"<code>code</code>",
		"<strong>bold</strong>",
		"<em>slanted</em>",
		`<a href="user.html">link</a>`,
		`<pre><code class="language-yaml"><span class="tok-key">kind</span><span class="tok-punct">:</span> User</code></pre>`,
		"<table>",
		`<td style="text-align:right">integer</td>`,
		"<li>first</li>",
		"<li>third</li>",
		"<blockquote>",
		"<hr>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output is missing %q\n%s", want, html)
		}
	}

	if strings.Count(html, "<ul>") != 2 {
		t.Errorf("expected an outer and a nested list, got:\n%s", html)
	}
	if len(headings) != 1 || headings[0].ID != "getting-started" || headings[0].Level != 2 {
		t.Errorf("headings = %+v", headings)
	}
}

func TestRenderMarkdownEscapesAndComments(t *testing.T) {
	html, _ := renderMarkdown("Angle < brackets & ampersands <!-- hidden -->\n\n<!-- whoctl:begin spec -->\nignored\n<!-- whoctl:end spec -->\n")
	if !strings.Contains(html, "Angle &lt; brackets &amp; ampersands") {
		t.Errorf("text was not escaped: %s", html)
	}
	if strings.Contains(html, "whoctl:begin") {
		t.Errorf("markers leaked into the output: %s", html)
	}
}

func TestRenderMarkdownDuplicateHeadings(t *testing.T) {
	_, headings := renderMarkdown("## Example\n\n## Example\n")
	if len(headings) != 2 || headings[0].ID != "example" || headings[1].ID != "example-2" {
		t.Errorf("anchors must be unique per page, got %+v", headings)
	}
}

func TestInlineHTMLLeavesLoneMarkersAlone(t *testing.T) {
	got := inlineHTML("2 * 3 * 4 and an [unclosed link")
	if !strings.Contains(got, "[unclosed link") {
		t.Errorf("unclosed link should survive as text: %s", got)
	}
}

func TestSplitRowHandlesEscapedPipes(t *testing.T) {
	cells := splitRow(`| a \| b | c |`)
	if len(cells) != 2 || strings.TrimSpace(cells[0]) != "a | b" {
		t.Errorf("splitRow = %q", cells)
	}
}
