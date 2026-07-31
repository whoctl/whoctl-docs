# whoctl-docs

The published site — `web/templates`, `web/assets`, and the builder that turns
every provider's documentation bundle into one static site — **and the registry
index whoctl installs providers from**.

No page here belongs to a provider: a provider ships prose and a schema, and
this repository turns them into a website. The index is the one thing here that
is part of the tool rather than about it, and it is here because it answers the
same question from the same file: which providers are there.

```sh
make build    # render site/ and site/registry/ from providers.yaml
make serve    # build it, then nginx on :8080
make local    # point the catalogue at sibling checkouts (no index: nothing is published)
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
provider's repository can put it on the site or take it off — nor, now, into the
registry: the same line carries `repository`, and removing it takes a provider
off the site and out of the index together. Documenting a version nobody can
install, and serving one nobody documented, are both worse than neither.

## The registry index

`internal/registry` writes `site/registry/<namespace>/<name>/…`, which is what
`whoctl get linux/users` reads on a machine with no provider installed. The
format is in `whoctl/internal/install/index.go`, in another repository, so
`TestIndexShape` pins the wire names — a rename here breaks installation
everywhere, and it breaks it at the far end, where the message is about a
missing checksum rather than about a rename. If the format has to change, it
belongs in the SDK first, where both sides can import one definition.

**The index is derived, never accumulated.** Every build asks the GitHub
releases API what exists right now and writes the answer whole, so a yanked
version disappears on the next build and `versions.json` cannot drift from what
is actually downloadable. That is also why the nightly schedule matters: it is
the only thing that notices a release published while a dispatch was failing.

**An archive with no checksum beside it is left out**, because whoctl refuses to
install what it cannot verify — publishing it in the index would only move the
failure to the user's machine.

**Signing is `WHOCTL_SIGNING_KEY`, and it is half of a pair.** The public half is
compiled into whoctl as `officialKeyHex`. Setting one without the other is what
breaks installation: an index signed by a key whoctl does not have is refused,
and whoctl with a key against an unsigned index is refused too. Generate the key
once, keep the private half in this repository's secrets and nowhere else, and
put the public half in whoctl in the same change.

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
