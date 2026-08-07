package web

import (
	"strings"
	"testing"
)

// The filter on the browse page hides a card by setting its `hidden` property.
// That works because the browser's own stylesheet says `[hidden] { display:
// none }` — and it stops working the moment any author rule sets `display` on
// the same element, which `.card { display: block }` does.
//
// # Why this is a test rather than a comment
//
// The failure is invisible from every direction that normally catches things:
// the markup is right, the script runs, the attribute is set, and the page
// looks exactly as it did before. Nothing but typing in the box and watching
// nothing happen will show it, and nobody types in the box while changing a
// colour.
func TestTheStylesheetMakesHiddenMeanHidden(t *testing.T) {
	css, err := FS.ReadFile("assets/whoctl.css")
	if err != nil {
		t.Fatalf("reading the stylesheet: %v", err)
	}
	if !strings.Contains(string(css), "[hidden]") {
		t.Error("no [hidden] rule: anything the browse filter hides will stay on screen")
	}
}

// The cards carry their own search terms, because the page is one static file
// and there is nothing to ask.
func TestTheBrowseCardsCarryTheirTerms(t *testing.T) {
	tpl, err := FS.ReadFile("templates/browse.html")
	if err != nil {
		t.Fatalf("reading the template: %v", err)
	}
	for _, want := range []string{`id="filter"`, "data-terms=", `id="no-match"`} {
		if !strings.Contains(string(tpl), want) {
			t.Errorf("the browse template lost %s, which the filter reads", want)
		}
	}
}
