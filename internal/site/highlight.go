package site

import (
	"fmt"
	"slices"
	"strings"
)

// Syntax highlighting for the fenced blocks the provider pages use.
//
// Same reasoning as the Markdown renderer next door: the site ships as static
// files with no external assets, so pulling in a JavaScript highlighter would
// mean either a CDN the page cannot reach offline or a vendored blob nobody
// reviews. The input is not arbitrary code either — it is the three languages
// the pages actually contain — so a tokenizer that covers those and falls back
// to plain text for anything else is the honest size for the problem.
//
// Every token becomes a <span class="tok-…"> and the colours live in
// whoctl.css, which means the same markup follows the light and dark themes.

// highlight renders a code block's body as HTML. Text is always escaped, so an
// unknown language degrades to exactly what the renderer emitted before.
func highlight(lang, code string) string {
	var t tokens
	switch strings.ToLower(lang) {
	case "yaml", "yml":
		for i, line := range strings.Split(code, "\n") {
			t.newline(i)
			yamlLine(&t, line)
		}
	case "sh", "bash", "shell", "zsh":
		for i, line := range strings.Split(code, "\n") {
			t.newline(i)
			shellLine(&t, line)
		}
	case "console", "shell-session", "terminal":
		for i, line := range strings.Split(code, "\n") {
			t.newline(i)
			consoleLine(&t, line)
		}
	default:
		return escapeHTML(code)
	}
	return t.String()
}

// tokens collects the highlighted output. Text goes through escapeHTML on the
// way in, so callers hand it raw source and never have to think about it.
type tokens struct {
	b strings.Builder
}

func (t *tokens) plain(s string) {
	t.b.WriteString(escapeHTML(s))
}

func (t *tokens) span(class, s string) {
	if s == "" {
		return
	}
	fmt.Fprintf(&t.b, "<span class=%q>%s</span>", "tok-"+class, escapeHTML(s))
}

func (t *tokens) newline(i int) {
	if i > 0 {
		t.b.WriteByte('\n')
	}
}

func (t *tokens) String() string { return t.b.String() }

// --- YAML -------------------------------------------------------------

func yamlLine(t *tokens, line string) {
	indent, rest := splitIndent(line)
	t.plain(indent)

	switch {
	case rest == "":
		return

	case strings.HasPrefix(rest, "#"):
		t.span("cmt", rest)
		return

	// Document markers, which the multi-document manifests are full of.
	case rest == "---" || rest == "...":
		t.span("punct", rest)
		return
	}

	// A list item is a dash and then, recursively, a whole line: an item can be
	// a scalar (`- wheel`) or the first field of a mapping (`- name: alice`).
	if rest == "-" || strings.HasPrefix(rest, "- ") {
		t.span("punct", "-")
		if len(rest) > 1 {
			yamlLine(t, rest[1:])
		}
		return
	}

	if key, value, ok := yamlKey(rest); ok {
		t.span("key", key)
		t.span("punct", ":")
		yamlValue(t, value)
		return
	}
	yamlValue(t, rest)
}

// yamlKey splits "key: value" when the line really is a mapping entry. The
// colon has to be followed by a space or end the line, so a plain scalar like
// `linux.whoctl.io/v1alpha1` is not mistaken for one.
func yamlKey(s string) (key, value string, ok bool) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ':':
			if i+1 == len(s) {
				return s[:i], "", true
			}
			if s[i+1] == ' ' {
				return s[:i], s[i+1:], true
			}
		case '#', '\'', '"':
			return "", "", false
		}
	}
	return "", "", false
}

func yamlValue(t *tokens, s string) {
	lead, rest := splitIndent(s)
	t.plain(lead)
	if rest == "" {
		return
	}

	// A trailing comment is the only thing that can follow a value.
	if i := strings.Index(rest, " #"); i >= 0 {
		yamlScalar(t, rest[:i])
		t.span("cmt", rest[i:])
		return
	}
	if strings.HasPrefix(rest, "#") {
		t.span("cmt", rest)
		return
	}
	yamlScalar(t, rest)
}

func yamlScalar(t *tokens, s string) {
	switch {
	case s == "":
	case strings.HasPrefix(s, `"`), strings.HasPrefix(s, "'"):
		t.span("str", s)
	case isNumber(s):
		t.span("num", s)
	case slices.Contains(yamlLiterals, s):
		t.span("lit", s)
	default:
		t.plain(s)
	}
}

// yamlLiterals are the unquoted scalars that are values rather than text.
var yamlLiterals = []string{"true", "false", "null", "~", "yes", "no", "[]", "{}"}

// --- shell ------------------------------------------------------------

// consoleLine highlights a transcript: the prompt and the command typed after
// it are shell, everything else is program output and stays plain, which is
// what makes the commands stand out in a block that is mostly output.
func consoleLine(t *tokens, line string) {
	prompt, rest, ok := splitPrompt(line)
	if !ok {
		t.plain(line)
		return
	}
	t.span("prompt", prompt)
	shellLine(t, rest)
}

// splitPrompt recognises "$ ", "# " and a prefixed form such as
// "whoctl-lab:~# ". The prefix may not contain spaces, so an output line that
// happens to hold a lone $ or # is left alone.
func splitPrompt(line string) (prompt, rest string, ok bool) {
	i := strings.IndexAny(line, "$#")
	if i < 0 || i+1 >= len(line) || line[i+1] != ' ' {
		return "", "", false
	}
	if strings.ContainsAny(line[:i], " \t") {
		return "", "", false
	}
	return line[:i+1], line[i+1:], true
}

// shellWords are the ones worth colouring in a one-line command; whoctl's
// examples are pipelines and redirections, not scripts with control flow.
var shellWords = []string{"cd", "do", "done", "echo", "else", "export", "fi", "for", "if", "in", "sudo", "then", "while"}

func shellLine(t *tokens, line string) {
	indent, rest := splitIndent(line)
	t.plain(indent)
	if rest == "" {
		return
	}
	if strings.HasPrefix(rest, "#") {
		t.span("cmt", rest)
		return
	}

	// start says whether the next word begins a command, which is what decides
	// between the "command" colour and a plain argument. It is true at the
	// beginning and again after every operator that starts a new one.
	start := true
	for i := 0; i < len(rest); {
		switch c := rest[i]; {
		case c == ' ' || c == '\t':
			j := i
			for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t') {
				j++
			}
			t.plain(rest[i:j])
			i = j

		case c == '\'' || c == '"':
			j := closingQuote(rest, i)
			t.span("str", rest[i:j])
			i, start = j, false

		case c == '#' && i > 0 && (rest[i-1] == ' ' || rest[i-1] == '\t'):
			t.span("cmt", rest[i:])
			return

		case strings.ContainsRune("|&;()<>", rune(c)):
			j := i
			for j < len(rest) && strings.ContainsRune("|&;()<>", rune(rest[j])) {
				j++
			}
			t.span("punct", rest[i:j])
			i, start = j, true

		default:
			j := i
			for j < len(rest) && !strings.ContainsAny(rest[j:j+1], " \t'\"|&;()<>") {
				j++
			}
			word := rest[i:j]
			switch {
			case strings.HasPrefix(word, "-") && word != "-":
				t.span("flag", word)
			case slices.Contains(shellWords, word):
				t.span("kw", word)
			case start:
				t.span("cmd", word)
			case isVariable(word):
				t.span("var", word)
			default:
				t.plain(word)
			}
			// A leading VAR=value assignment does not consume the command slot.
			i, start = j, start && strings.Contains(word, "=")
		}
	}
}

// closingQuote returns the index just past the quote that closes the one at i,
// or the end of the line when it is never closed.
func closingQuote(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		if s[j] == '\\' {
			j++
			continue
		}
		if s[j] == q {
			return j + 1
		}
	}
	return len(s)
}

// --- shared -----------------------------------------------------------

func splitIndent(s string) (indent, rest string) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i], s[i:]
}

func isVariable(s string) bool {
	return strings.HasPrefix(s, "$") && len(s) > 1
}

func isNumber(s string) bool {
	s = strings.TrimPrefix(s, "-")
	if s == "" {
		return false
	}
	dots := 0
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c == '.':
			dots++
			if dots > 1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
