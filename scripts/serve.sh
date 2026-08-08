#!/bin/sh
# Builds the documentation site and serves it with nginx in a container.
#
# The site is static and works from a file:// path too, but a server is what
# makes the links between providers and the "On this page" anchors behave like
# they will once published — and it reloads on refresh, so `make build` in
# another terminal is enough to see a change.
#
# Usage:
#   scripts/serve.sh              # build, then serve on http://localhost:8080
#   DOCS_PORT=9000 scripts/serve.sh
#   SKIP_BUILD=1 scripts/serve.sh # serve site as it already stands
#
# Environment:
#   DOCS_PORT         port on the host (default 8080)
#   OUTPUT            site directory (default site)
#   SKIP_BUILD        set to 1 to skip regenerating the site
#   CONTAINER_ENGINE  podman (default) or docker
#   NGINX_IMAGE       image to run (default nginx:alpine)
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
engine="${CONTAINER_ENGINE:-podman}"
image="${NGINX_IMAGE:-nginx:alpine}"
port="${DOCS_PORT:-8080}"
output="${OUTPUT:-site}"

if ! command -v "$engine" >/dev/null 2>&1; then
	echo "container engine '$engine' not found; set CONTAINER_ENGINE" >&2
	exit 1
fi

if [ "${SKIP_BUILD:-}" != "1" ]; then
	( cd "$root" && make --no-print-directory build ) >&2
fi

site="$root/$output"
if [ ! -f "$site/index.html" ]; then
	# Single quotes around the command: in double quotes those backticks are
	# command substitution, so this message ran `make build` instead of naming
	# it — in the one path where the site is missing, which is exactly when
	# building unasked is least welcome.
	echo "no site at $site; run 'make build' first" >&2
	exit 1
fi

echo
echo "serving $output on http://localhost:$port  (ctrl-c to stop)"
echo

# Only allocate a TTY when there is one, so this works from scripts and CI too.
tty_flags=""
if [ -t 0 ] && [ -t 1 ]; then
	tty_flags="-it"
fi

# The site is mounted read-only: this container serves the documentation, it
# never writes it. --security-opt label=disable keeps SELinux from blocking the
# mount on Fedora without relabelling files in the repository.
exec "$engine" run --rm $tty_flags \
	--name whoctl-docs \
	--security-opt label=disable \
	-p "$port:80" \
	-v "$site:/usr/share/nginx/html:ro" \
	"$image"
