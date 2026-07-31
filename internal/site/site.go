package site

import (
	"bytes"
	"fmt"
	"github.com/whoctl/whoctl-sdk-go/docs"
	"html/template"
	"path"
	"sort"
	"strings"

	"github.com/whoctl/whoctl-docs/web"
	"github.com/whoctl/whoctl-sdk-go/schema"
)

// The two helpers the templates need. They are functions because their
// receivers now live in the SDK — see Groupings.
var siteFuncs = template.FuncMap{
	"groupings": Groupings,
	"names":     Names,
}

var siteTemplates = template.Must(
	template.New("site").Funcs(siteFuncs).ParseFS(web.FS, "templates/*.html"))

// Render builds the whole site and returns it as a set of files, keyed by
// their path relative to the output directory. Returning files rather than
// writing them keeps the renderer testable and leaves the decision of where
// they land to the caller.
func Render(site *docs.Site) (map[string][]byte, error) {
	out := map[string][]byte{}

	css, err := web.FS.ReadFile("assets/whoctl.css")
	if err != nil {
		return nil, err
	}
	out["assets/whoctl.css"] = css

	if err := renderPage(out, "index.html", "browse.html", pageData{
		Site:  site,
		Title: site.Title,
	}); err != nil {
		return nil, err
	}

	for i := range site.Providers {
		p := &site.Providers[i]
		if err := renderProvider(out, site, p); err != nil {
			return nil, fmt.Errorf("provider %s: %w", p.Name, err)
		}
	}
	return out, nil
}

func renderProvider(out map[string][]byte, site *docs.Site, p *docs.Provider) error {
	base := path.Join("providers", p.Name)

	overview, headings := renderPageBody(p.Overview.Body)
	err := renderPage(out, path.Join(base, "index.html"), "provider.html", pageData{
		Site:     site,
		Title:    p.DisplayName + " provider — " + site.Title,
		Provider: p,
		Tab:      "overview",
		Content:  overview,
		Headings: headings,
	})
	if err != nil {
		return err
	}

	err = renderPage(out, path.Join(base, "docs", "index.html"), "doclist.html", pageData{
		Site:     site,
		Title:    "Documentation — " + p.DisplayName,
		Provider: p,
		Tab:      "docs",
	})
	if err != nil {
		return err
	}

	for i := range p.Resources {
		r := &p.Resources[i]
		content, headings := resourceContent(*r)
		err := renderPage(out, path.Join(base, "docs", r.Slug+".html"), "resource.html", pageData{
			Site:     site,
			Title:    r.Kind + " — " + p.DisplayName + " provider",
			Provider: p,
			Resource: r,
			Tab:      "docs",
			Content:  content,
			Headings: headings,
		})
		if err != nil {
			return err
		}
	}

	for i := range p.Guides {
		g := &p.Guides[i]
		content, headings := renderPageBody(g.Body)
		err := renderPage(out, path.Join(base, "guides", g.Slug+".html"), "guide.html", pageData{
			Site:     site,
			Title:    g.Title + " — " + p.DisplayName + " provider",
			Provider: p,
			Guide:    g,
			Tab:      "guides",
			Content:  content,
			Headings: headings,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// pageData is what every template sees.
type pageData struct {
	Site     *docs.Site
	Title    string
	Provider *docs.Provider
	Resource *docs.Resource
	Guide    *docs.Page
	Tab      string
	Content  template.HTML
	Headings []Heading

	// Root is the relative path back to the site root, so the site can be
	// opened straight off the filesystem without a server.
	Root string
	// Body is the rendered page, dropped into the layout.
	Body template.HTML
}

func renderPage(out map[string][]byte, file, tpl string, data pageData) error {
	data.Root = rootPrefix(file)

	var body bytes.Buffer
	if err := siteTemplates.ExecuteTemplate(&body, tpl, data); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	data.Body = template.HTML(body.String())

	var page bytes.Buffer
	if err := siteTemplates.ExecuteTemplate(&page, "layout.html", data); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	out[file] = page.Bytes()
	return nil
}

// rootPrefix is the "../" chain that leads from a page back to the root.
func rootPrefix(file string) string {
	return strings.Repeat("../", strings.Count(file, "/"))
}

func renderPageBody(md string) (template.HTML, []Heading) {
	if strings.TrimSpace(md) == "" {
		return "", nil
	}
	html, headings := renderMarkdown(md)
	return template.HTML(html), headings
}

// resourceContent renders a resource page: the prose as written, with the
// generated sections rendered in place of their markers. A page that never
// placed the spec or status sections gets them appended, so no kind is left
// with its fields undocumented just because somebody forgot a marker.
func resourceContent(r docs.Resource) (template.HTML, []Heading) {
	rend := &renderer{}
	var b strings.Builder

	placed := map[string]bool{}
	for _, seg := range docs.SplitSegments(r.Body) {
		if seg.Section == "" {
			b.WriteString(rend.blocks(splitLines(seg.Markdown)))
			continue
		}
		placed[seg.Section] = true
		b.WriteString(sectionHTML(rend, seg.Section, r))
	}

	if !placed[docs.SectionSpec] {
		rend.heading(&b, "## Spec", 2)
		b.WriteString(fieldsHTML(r.Spec, true))
	}
	if !placed[docs.SectionStatus] && len(r.Status) > 0 {
		rend.heading(&b, "## Status", 2)
		b.WriteString(fieldsHTML(r.Status, false))
	}
	return template.HTML(b.String()), rend.headings
}

func sectionHTML(rend *renderer, name string, r docs.Resource) string {
	switch name {
	case docs.SectionSpec:
		return fieldsHTML(r.Spec, true)
	case docs.SectionStatus:
		return fieldsHTML(r.Status, false)
	default:
		// meta and columns are plain tables; rendering the same markdown the
		// source page carries keeps the site and the file in agreement.
		return rend.blocks(splitLines(docs.SectionMarkdown(name, r)))
	}
}

// fieldsHTML lays a spec or status out as a list of fields, the way a
// reference page reads: name and type first, then the markers, then the prose.
func fieldsHTML(fields []schema.Field, spec bool) string {
	if len(fields) == 0 {
		return "<p class=\"empty\">This kind has no fields.</p>\n"
	}
	var b strings.Builder
	b.WriteString("<div class=\"fields\">\n")
	for _, f := range fields {
		b.WriteString("<div class=\"field\">\n<div class=\"field-head\">")
		fmt.Fprintf(&b, "<code class=\"field-name\">%s</code>", escapeHTML(f.Name))
		fmt.Fprintf(&b, "<span class=\"field-type\">%s</span>", escapeHTML(f.Type))
		switch {
		case !spec:
			b.WriteString(`<span class="tag">read-only</span>`)
		case f.Optional:
			b.WriteString(`<span class="tag">optional</span>`)
		default:
			b.WriteString(`<span class="tag required">required</span>`)
		}
		for _, label := range docs.FlagLabels(f) {
			fmt.Fprintf(&b, `<span class="tag flag">%s</span>`, escapeHTML(label))
		}
		b.WriteString("</div>\n")

		if doc := strings.TrimSpace(f.Doc); doc != "" {
			fmt.Fprintf(&b, "<p class=\"field-doc\">%s</p>\n", inlineHTML(doc))
		} else {
			b.WriteString("<p class=\"field-doc missing\">Undocumented.</p>\n")
		}
		if f.Example != "" {
			fmt.Fprintf(&b, "<p class=\"field-example\">Example: <code>%s</code></p>\n", escapeHTML(f.Example))
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("</div>\n")
	return b.String()
}

// Grouping is a subcategory of a provider's resources, as shown in the
// sidebar.
type Grouping struct {
	Name      string
	Slug      string
	Resources []docs.Resource
}

// Groupings buckets the provider's resources by subcategory.
//
// It is a function rather than a method because docs.Provider belongs to the
// SDK: how a provider's kinds are arranged on a page is this repository's
// business, not the SDK's, and the templates reach it through the func map.
func Groupings(p docs.Provider) []Grouping {
	var order []string
	byName := map[string][]docs.Resource{}
	for _, r := range p.Resources {
		name := r.Subcategory
		if name == "" {
			name = docs.DefaultSubcategory
		}
		if _, seen := byName[name]; !seen {
			order = append(order, name)
		}
		byName[name] = append(byName[name], r)
	}
	sort.Strings(order)

	out := make([]Grouping, 0, len(order))
	for _, name := range order {
		out = append(out, Grouping{Name: name, Slug: slug(name), Resources: byName[name]})
	}
	return out
}

// Names lists every way the resource can be typed on the command line.
func Names(r docs.Resource) []string {
	names := []string{r.Plural, r.Singular}
	return append(names, r.ShortNames...)
}

func slug(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteRune(c)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
