# whoctl-docs

The whoctl documentation site: templates, styles, and the builder that turns
every provider's published documentation into one static site, served by GitHub
Pages. The same build writes the **registry index** whoctl installs providers
from, and Pages serves that too.

`whoctl` ships no templates and no CSS; a provider ships prose and a schema, and
this repository turns the second into a website. The index is the one thing here
that is part of the tool rather than about it — it is here because it answers
the same question from the same file: which providers are there.

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
| `providers.yaml` | Which providers the site covers, at which versions, and which repositories publish them. |
| `web/templates` | The HTML the site is rendered into. |
| `web/assets` | One stylesheet. See below. |
| `internal/site` | Reading the catalogue and the bundles, markdown to HTML, the resource pages, the browse index, and the syntax highlighter. |
| `internal/registry` | The registry index: what whoctl installs from. |
| `cmd/whoctl-docs` | The builder. |
| `cmd/keygen` | Generates the signing key pair, once, by hand. |

```sh
make build            # site/ and site/registry/
make local && make build   # against provider checkouts beside this one
```

## The registry index

`whoctl get linux/users` on a machine with no provider installed reads two small
files:

```
/registry/whoctl/linux/versions.json
/registry/whoctl/linux/0.1.0/linux_amd64.json
```

The second says where the archive is, what it hashes to, and — once a key is
configured — who signed it. whoctl refuses to install anything it cannot verify
against that, so an archive published with no checksum beside it is left out of
the index rather than offered and rejected on somebody's machine.

**The index is derived from the GitHub releases API, not accumulated.** Every
build asks what is published right now and writes the answer whole, so a yanked
release disappears on the next build. `repository` in a catalogue entry is what
makes a provider installable; an entry without one is documented and not
indexed, which is exactly what `make local` produces.

### Publishing

`.github/workflows/pages.yml` builds and deploys on a push here, on the nightly
schedule, and on a `repository_dispatch` that each provider's release sends. Two
secrets, both optional and both with consequences:

| Secret | Where | Without it |
| --- | --- | --- |
| `WHOCTL_DOCS_TOKEN` | each provider repository | A release does not notify the site; it catches up on the nightly run. |
| `WHOCTL_SIGNING_KEY` | here | The index is published unsigned, and providers are verified by checksum alone. |

A checksum published by whoever published the binary proves the download was not
corrupted in transit and nothing else. Only the signature says who published it.
Generate the key once, keep the private half here and nowhere else, and put the
public half in whoctl's `officialKeyHex` in the same change — they are a pair,
and setting either alone stops installation:

```sh
make keygen
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
