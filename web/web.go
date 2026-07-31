// Package web is the presentation half of the site: the HTML it is rendered
// into and the one stylesheet it carries.
//
// It sits at the top of the repository rather than under internal/ because in a
// documentation repository the templates are the product. A provider ships
// prose and a schema; everything about how that looks is here, in one copy, so
// changing it changes the whole site at once — which is the reason the site is
// built centrally instead of by each provider.
package web

import "embed"

// FS holds the templates and the assets.
//
// **No external assets.** One stylesheet, no fonts, no scripts, so a page works
// from a file:// path and from a web server alike.
//
//go:embed templates/*.html assets/*
var FS embed.FS
