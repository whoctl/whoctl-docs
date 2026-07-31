# whoctl-docs

The whoctl documentation site: templates, styles, and the builder that turns
every provider's published documentation into one static site, served by GitHub
Pages.

Nothing here is part of the tool. `whoctl` ships no templates and no CSS; a
provider ships prose and a schema. This repository is what turns the second into
a website.

## How a page gets here

1. A provider repository writes its prose by hand in `docs/`, and the SDK
   generates the field tables from the `doc` tags on its spec and status
   structs.
2. Its release publishes a **docs bundle** — `whoctl-provider-x --docs-bundle` —
   which is those pages plus the schema they were generated from, as JSON, as a
   release artifact.
3. `providers.yaml` here lists which providers, at which versions, the site
   covers.
4. The builder fetches those bundles, renders them against the templates in this
   repository, and writes `site/`.

A provider is added to the site by adding a line to `providers.yaml`, and that
is the whole of the site's editorial control: nothing in a provider's own
repository can put it on the site or take it off. There is no page to write
here, and no template a provider can override — a provider that could ship
markup could ship script, and what it ships is markdown and a schema.

`bundle` in a catalogue entry is a URL or a path, so the workspace builds the
site from providers checked out beside it before anything is published.

## Layout

| Path | Role |
| --- | --- |
| `providers.yaml` | Which providers the site covers, and at which versions. |
| `web/templates` | The HTML the site is rendered into. |
| `web/assets` | One stylesheet. See below. |
| `internal/site` | Reading the catalogue and the bundles, markdown to HTML, the resource pages, the browse index, and the syntax highlighter. |
| `cmd/whoctl-docs` | The builder. |

```sh
go run ./cmd/whoctl-docs -o site
```

## The site has no external assets

One stylesheet, no fonts, no scripts, so a page works from a `file://` path and
from a web server alike. That is why syntax highlighting is a tokenizer rather
than a JavaScript library: it covers the three languages the pages actually use
— `yaml`, `sh` and `console` — emits `<span class="tok-…">` and lets the CSS
follow the light and dark themes. An unknown language falls back to escaped
plain text, so a new fence can never render as broken markup.

## Versions

Bundles are versioned, so the site can carry more than one version of a
provider's pages and a URL can be stable:

```
/providers/linux/v0.3.1/resources/user/
/providers/linux/latest/resources/user/
```
