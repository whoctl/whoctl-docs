# whoctl-docs

The published site: `web/templates`, `web/assets`, and the builder that turns
every provider's documentation bundle into one static site.

Nothing here is part of the tool, and no page here belongs to a provider. A
provider ships prose and a schema; this repository turns them into a website.

```sh
make build    # render site/ from the bundles in providers.yaml
make serve    # build it, then nginx on :8080
```

A bundle comes from the provider's own repository — `make docs` there. The
`bundle` field of a catalogue entry takes a path as well as a URL, so the
workspace renders the site from siblings before anything is published.

## Decisions somebody would otherwise undo

**The site is built centrally, and that is the whole design.** The alternative —
every provider building and hosting its own — was considered and rejected
twice, for reasons that still hold:

- Whoever owns the templates owns the look. Distributed templates means copies
  in every repository, or templates shipped by the SDK; and a Go dependency is
  *pinned*, so changing a colour would reach nobody until four repositories bump
  and re-release. In the meantime the site has four appearances.
- A provider ships content, not markup. Markdown and a schema cannot inject
  script into a page; rendered HTML from a repository we do not control can.
  That costs nothing today and is the whole ballgame once somebody else's
  provider is on the site.

**`providers.yaml` is the only editorial control**, and it is here. Nothing in a
provider's repository can put it on the site or take it off.

**No external assets.** One stylesheet, no fonts, no scripts, so a page works
from a `file://` path and from a web server alike. That is why syntax
highlighting is a tokenizer rather than a JavaScript library: it covers the
three languages the pages actually use — `yaml`, `sh` and `console` — and an
unknown language falls back to escaped plain text, so a new fence can never
render as broken markup.

**One unreadable bundle fails the build.** A site quietly missing a provider is
much harder to notice than a build that stopped.

**Presentation stays here, content stays in the SDK.** How `writeOnly` is
worded for a reader belongs to the site that renders it; what the marker *is*
belongs to the schema that declares it. `Groupings` and `Names` are template
funcs for the same reason — their receivers are the SDK's types, and how a
provider's kinds are arranged on a page is this repository's business.
