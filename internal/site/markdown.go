package site

import (
	"fmt"
	"strings"
	"unicode"
)

// A small Markdown renderer, deliberately not a general one.
//
// whoctl has two dependencies, cobra and yaml, and a documentation site is a
// poor reason for a third: the input here is not arbitrary Markdown from the
// internet, it is the pages that live in this repository, in the subset the
// provider pages actually use — headings, paragraphs, fenced code, tables,
// lists, block quotes and rules. `whoctl docs check` renders every page, so
// anything this cannot parse is caught by the test suite rather than silently
// mangled in the published site.

// Heading is one entry of a page's table of contents.
type Heading struct {
	Level int
	Text  string
	ID    string
}

// renderMarkdown converts a page to HTML and returns the headings it found, in
// document order, for the "On this page" navigation.
func renderMarkdown(src string) (string, []Heading) {
	r := &renderer{}
	html := r.blocks(splitLines(src))
	return html, r.headings
}

type renderer struct {
	headings []Heading
	// seen makes heading anchors unique within a page, so two "Example"
	// sections do not both answer to #example.
	seen map[string]int
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func (r *renderer) blocks(lines []string) string {
	var b strings.Builder
	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "":
			i++

		case strings.HasPrefix(trimmed, "<!--"):
			i = skipComment(lines, i)

		case isFence(trimmed):
			i = r.code(&b, lines, i)

		case strings.HasPrefix(trimmed, "#"):
			if n := headingLevel(trimmed); n > 0 {
				r.heading(&b, trimmed, n)
				i++
				break
			}
			i = r.paragraph(&b, lines, i)

		case isRule(trimmed):
			b.WriteString("<hr>\n")
			i++

		case strings.HasPrefix(trimmed, ">"):
			i = r.quote(&b, lines, i)

		case isTableStart(lines, i):
			i = r.table(&b, lines, i)

		case itemMarker(line) != "":
			i = r.list(&b, lines, i)

		default:
			i = r.paragraph(&b, lines, i)
		}
	}
	return b.String()
}

// skipComment swallows an HTML comment. Comments are how generated sections
// are delimited in the source pages, and they have no place in the output.
func skipComment(lines []string, i int) int {
	for ; i < len(lines); i++ {
		if strings.Contains(lines[i], "-->") {
			return i + 1
		}
	}
	return i
}

func isFence(s string) bool { return strings.HasPrefix(s, "```") || strings.HasPrefix(s, "~~~") }

func isRule(s string) bool {
	return s == "---" || s == "***" || s == "___"
}

func headingLevel(s string) int {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(s) || s[n] != ' ' {
		return 0
	}
	return n
}

func (r *renderer) heading(b *strings.Builder, line string, level int) {
	text := strings.TrimSpace(line[level:])
	id := r.anchor(text)
	r.headings = append(r.headings, Heading{Level: level, Text: text, ID: id})
	fmt.Fprintf(b, "<h%d id=%q>%s</h%d>\n", level, id, inlineHTML(text), level)
}

func (r *renderer) anchor(text string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(c) || unicode.IsDigit(c):
			b.WriteRune(c)
		case c == ' ' || c == '-' || c == '_' || c == '.':
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "section"
	}
	if r.seen == nil {
		r.seen = map[string]int{}
	}
	r.seen[slug]++
	if n := r.seen[slug]; n > 1 {
		slug = fmt.Sprintf("%s-%d", slug, n)
	}
	return slug
}

func (r *renderer) code(b *strings.Builder, lines []string, i int) int {
	open := strings.TrimSpace(lines[i])
	marker := open[:3]
	lang := strings.TrimSpace(strings.TrimLeft(open, "`~"))
	var body []string
	i++
	for ; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), marker) {
			i++
			break
		}
		body = append(body, lines[i])
	}
	class := ""
	if lang != "" {
		class = fmt.Sprintf(" class=%q", "language-"+lang)
	}
	fmt.Fprintf(b, "<pre><code%s>%s</code></pre>\n", class, highlight(lang, strings.Join(body, "\n")))
	return i
}

func (r *renderer) quote(b *strings.Builder, lines []string, i int) int {
	var body []string
	for ; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(t, ">") {
			break
		}
		body = append(body, strings.TrimPrefix(strings.TrimPrefix(t, ">"), " "))
	}
	fmt.Fprintf(b, "<blockquote>\n%s</blockquote>\n", r.blocks(body))
	return i
}

func (r *renderer) paragraph(b *strings.Builder, lines []string, i int) int {
	var body []string
	for ; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "" || isFence(t) || isRule(t) || headingLevel(t) > 0 ||
			strings.HasPrefix(t, ">") || strings.HasPrefix(t, "<!--") ||
			itemMarker(lines[i]) != "" || isTableStart(lines, i) {
			break
		}
		body = append(body, t)
	}
	if len(body) > 0 {
		fmt.Fprintf(b, "<p>%s</p>\n", inlineHTML(strings.Join(body, " ")))
	}
	return i
}

// itemMarker returns the bullet or number that starts a list item, or "" when
// the line is not one.
func itemMarker(line string) string {
	t := strings.TrimLeft(line, " ")
	if len(t) > 1 && (t[0] == '-' || t[0] == '*' || t[0] == '+') && t[1] == ' ' {
		return t[:1]
	}
	for n := 0; n < len(t); n++ {
		if t[n] >= '0' && t[n] <= '9' {
			continue
		}
		if n > 0 && t[n] == '.' && n+1 < len(t) && t[n+1] == ' ' {
			return t[:n+1]
		}
		break
	}
	return ""
}

func indentOf(line string) int { return len(line) - len(strings.TrimLeft(line, " ")) }

// list renders a bullet or numbered list, including nested ones: a line
// indented past the item's marker belongs to that item and is rendered as
// blocks of its own.
func (r *renderer) list(b *strings.Builder, lines []string, i int) int {
	base := indentOf(lines[i])
	ordered := !strings.ContainsAny(itemMarker(lines[i]), "-*+")

	type item struct{ body []string }
	var items []item

	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			// A blank line ends the list unless the next line continues it.
			if i+1 < len(lines) && indentOf(lines[i+1]) > base && strings.TrimSpace(lines[i+1]) != "" {
				if len(items) > 0 {
					items[len(items)-1].body = append(items[len(items)-1].body, "")
				}
				i++
				continue
			}
			break
		}
		marker := itemMarker(line)
		switch {
		case marker != "" && indentOf(line) == base:
			text := strings.TrimSpace(line)[len(marker):]
			items = append(items, item{body: []string{strings.TrimSpace(text)}})
			i++
		case indentOf(line) > base && len(items) > 0:
			last := &items[len(items)-1]
			last.body = append(last.body, line[min(indentOf(line), base+len(marker)+1):])
			i++
		default:
			// Same indentation but not an item: the list is over.
			return i
		}
	}

	tag := "ul"
	if ordered {
		tag = "ol"
	}
	fmt.Fprintf(b, "<%s>\n", tag)
	for _, it := range items {
		b.WriteString("<li>")
		if nested(it.body) {
			b.WriteString("\n")
			b.WriteString(r.blocks(it.body))
		} else {
			b.WriteString(inlineHTML(strings.Join(trimEmpty(it.body), " ")))
		}
		b.WriteString("</li>\n")
	}
	fmt.Fprintf(b, "</%s>\n", tag)
	return i
}

// nested reports whether an item's body holds a block of its own — a nested
// list or a code fence — as opposed to being plain wrapped text.
func nested(body []string) bool {
	for _, l := range body[1:] {
		if itemMarker(l) != "" || isFence(strings.TrimSpace(l)) {
			return true
		}
	}
	return false
}

func trimEmpty(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// isTableStart reports whether a pipe table begins at line i: a header row
// followed by the |---|---| separator.
func isTableStart(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
		return false
	}
	sep := strings.TrimSpace(lines[i+1])
	if !strings.HasPrefix(sep, "|") {
		return false
	}
	for _, cell := range splitRow(sep) {
		cell = strings.TrimSpace(cell)
		if cell == "" || strings.Trim(cell, "-: ") != "" {
			return false
		}
	}
	return true
}

func (r *renderer) table(b *strings.Builder, lines []string, i int) int {
	header := splitRow(lines[i])
	aligns := alignments(splitRow(lines[i+1]))
	i += 2

	b.WriteString("<div class=\"table-wrap\">\n<table>\n<thead>\n<tr>")
	for n, cell := range header {
		fmt.Fprintf(b, "<th%s>%s</th>", alignAttr(aligns, n), inlineHTML(strings.TrimSpace(cell)))
	}
	b.WriteString("</tr>\n</thead>\n<tbody>\n")
	for ; i < len(lines); i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
			break
		}
		b.WriteString("<tr>")
		for n, cell := range splitRow(lines[i]) {
			fmt.Fprintf(b, "<td%s>%s</td>", alignAttr(aligns, n), inlineHTML(strings.TrimSpace(cell)))
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody>\n</table>\n</div>\n")
	return i
}

// splitRow cuts a table row on unescaped pipes, so a cell can hold a literal
// one as \|.
func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")

	var cells []string
	var cur strings.Builder
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '|':
			cur.WriteByte('|')
			i++
		case line[i] == '|':
			cells = append(cells, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(line[i])
		}
	}
	cells = append(cells, cur.String())
	return cells
}

func alignments(sep []string) []string {
	out := make([]string, len(sep))
	for i, cell := range sep {
		cell = strings.TrimSpace(cell)
		left, right := strings.HasPrefix(cell, ":"), strings.HasSuffix(cell, ":")
		switch {
		case left && right:
			out[i] = "center"
		case right:
			out[i] = "right"
		}
	}
	return out
}

func alignAttr(aligns []string, i int) string {
	if i < len(aligns) && aligns[i] != "" {
		return fmt.Sprintf(" style=\"text-align:%s\"", aligns[i])
	}
	return ""
}

func escapeHTML(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	).Replace(s)
}

// inlineHTML renders the inline span syntax: code, links, bold, italic, and
// backslash escapes. Everything else is escaped as text.
func inlineHTML(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '\\' && i+1 < len(s):
			writeEscapedByte(&b, s[i+1])
			i += 2

		case c == '`':
			n := runLength(s, i, '`')
			if end := strings.Index(s[i+n:], s[i:i+n]); end >= 0 {
				code := s[i+n : i+n+end]
				fmt.Fprintf(&b, "<code>%s</code>", escapeHTML(strings.Trim(code, " ")))
				i += n + end + n
				continue
			}
			b.WriteString(escapeHTML(s[i : i+n]))
			i += n

		case c == '[':
			if text, url, size, ok := link(s, i); ok {
				fmt.Fprintf(&b, "<a href=%q>%s</a>", escapeHTML(pageURL(url)), inlineHTML(text))
				i += size
				continue
			}
			b.WriteString("[")
			i++

		case c == '*' && strings.HasPrefix(s[i:], "**"):
			if end := strings.Index(s[i+2:], "**"); end >= 0 {
				fmt.Fprintf(&b, "<strong>%s</strong>", inlineHTML(s[i+2:i+2+end]))
				i += end + 4
				continue
			}
			b.WriteString("**")
			i += 2

		case c == '*':
			if end := strings.Index(s[i+1:], "*"); end > 0 {
				fmt.Fprintf(&b, "<em>%s</em>", inlineHTML(s[i+1:i+1+end]))
				i += end + 2
				continue
			}
			b.WriteString("*")
			i++

		default:
			writeEscapedByte(&b, c)
			i++
		}
	}
	return b.String()
}

// writeEscapedByte copies one byte through, escaping it if HTML needs it.
//
// The obvious spelling — escapeHTML(string(c)) — is wrong on any character
// outside ASCII: c is a byte, and string(byte) widens it to a rune, so each
// continuation byte of a multi-byte character is re-encoded on its own. An em
// dash came out as "â€"", and so did every accented letter and curly quote in
// every page on the site. The four characters HTML escapes are all ASCII, so a
// byte that is none of them is copied untouched and multi-byte sequences
// survive whole.
func writeEscapedByte(b *strings.Builder, c byte) {
	switch c {
	case '&':
		b.WriteString("&amp;")
	case '<':
		b.WriteString("&lt;")
	case '>':
		b.WriteString("&gt;")
	case '"':
		b.WriteString("&quot;")
	default:
		b.WriteByte(c)
	}
}

func runLength(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// pageURL rewrites a link to a sibling page.
//
// A provider's prose is read in two places: on the site, and on GitHub, where
// the .md files sit next to each other. It is written for the second — that is
// where the author is looking — so a cross-reference is "[apk](apkpackage.md)",
// and publishing that verbatim gives the site a link to a file it does not
// serve. The page is rendered to .html, so the link is too.
//
// Only a relative link is touched: an absolute URL is somebody else's site, and
// a fragment is this page.
func pageURL(url string) string {
	if strings.Contains(url, "://") || strings.HasPrefix(url, "#") || strings.HasPrefix(url, "/") {
		return url
	}
	path, fragment, hasFragment := strings.Cut(url, "#")
	if !strings.HasSuffix(path, ".md") {
		return url
	}
	path = strings.TrimSuffix(path, ".md") + ".html"
	if hasFragment {
		return path + "#" + fragment
	}
	return path
}

// link parses [text](url) starting at i, allowing balanced brackets in the
// text. size is how many bytes the whole link takes.
func link(s string, i int) (text, url string, size int, ok bool) {
	depth := 0
	for n := i; n < len(s); n++ {
		switch s[n] {
		case '[':
			depth++
		case ']':
			depth--
			if depth > 0 {
				continue
			}
			if n+1 >= len(s) || s[n+1] != '(' {
				return "", "", 0, false
			}
			end := strings.IndexByte(s[n+2:], ')')
			if end < 0 {
				return "", "", 0, false
			}
			return s[i+1 : n], strings.TrimSpace(s[n+2 : n+2+end]), n + 2 + end + 1 - i, true
		}
	}
	return "", "", 0, false
}
