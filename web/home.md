## Install

```sh
curl -fsSL https://raw.githubusercontent.com/whoctl/whoctl/main/install.sh | sh
```

On Windows, take `whoctl-windows-amd64.exe` from the
[releases page](https://github.com/whoctl/whoctl/releases/latest).

## The first command installs what it needs

whoctl ships with no provider. The first command that says `linux/` fetches the
linux provider, verifies its checksum and signature, and answers:

```console
$ whoctl get linux/users
Installing whoctl/linux 0.1.0  ████████████████░░░░░░░░  8.2/12.4 MB
NAME     UID     GID     GROUP    SHELL
root     0       0       root     /bin/ash
alice    1000    1000    alice    /bin/sh
bob      1001    1001    bob      /bin/bash
nobody   65534   65534   nobody   /sbin/nologin
```

Providers live under `~/.whoctl`. `whoctl upgrade` moves them forward;
`--offline` uses only what is there and never reaches the network.

## A resource always carries its provider

The prefix is not optional:

```console
$ whoctl get users
error: resource "users" needs its provider: try linux/users
```

`whoctl get users` does not say *whose* users, and once two providers serve a
kind of the same name the guess whoctl would have to make is the kind of mistake
that deletes the wrong account. Within that, the plural, the singular and any
short name all work — `linux/users`, `linux/user`, `linux/usr`.

A provider covering a cloud groups its kinds the way that cloud does, and the
group sits between the provider and the resource — `aws/route53/hostedzones`.
The group can be left out while nothing else answers to the name, and when
something does, the answer names both rather than guessing:

```console
$ whoctl get aws/instances
error: resource "aws/instances" is ambiguous between aws/ec2/instances, aws/rds/instances
```

## Reading

`get` lists and shows; `describe` is the long form.

```console
$ whoctl get linux/user alice -o yaml
apiVersion: linux.whoctl.io/v1alpha1
kind: User
metadata:
  name: alice
spec:
  uid: 1000
  primaryGroup: alice
  groups:
    - developers
    - wheel
  shell: /bin/sh
  home: /home/alice
  comment: Alice Liddell
  locked: false
status:
  uid: 1000
  gid: 1000
  homeExists: true
  system: false
  passwordSet: true
```

`spec` is the observed state, not a wish: what `get` hands back is what `apply`
accepts, so the round trip reports `unchanged` rather than rewriting anything.
`status` is what the system says and no manifest can set.

`-o name` prints the fully qualified reference of each object, and every verb
accepts that same string — so one command's output is the next one's input:

```console
$ whoctl get linux/groups -o name
linux/group/developers

$ whoctl describe linux/group/developers
```

### Narrowing what comes back

The four flags Kubernetes uses, meaning what they mean there:

```console
$ whoctl get coredns/servers -l coredns.whoctl.io/port=5353
NAME                 ZONES            PORT   PLUGINS   UPSTREAM   AGE
internal.test-5353   internal.test.   5353   3         -          1h

$ whoctl get coredns/zones --field-selector status.records=3
NAME          RECORDS   SERIAL       TTL   SERVERS          AGE
example.org   3         2026080502   300   example.com-53   1h
```

`-l` matches labels and `--field-selector` matches a path spelled the way the
manifest spells it. A provider *may* push either down — the aws provider turns
`status.state=running` into an EC2 filter, so the query is smaller rather than
the answer — but whoctl applies both again to whatever comes back. A provider
that ignores them is slower and never wrong, and the flags mean the same thing
against a provider whoctl has never seen.

`-n` picks a namespace and `-A` asks for every one. What a namespace *is* stays
the provider's business: a region for aws, nothing at all for a kind that is
global. Neither flag says anything to a kind that has no namespace.

```console
$ whoctl get aws/ec2/instances -A
NAME                  REGION      TYPE       STATE     PRIVATE-IP       AGE
i-979bf6bd5634adbc5   us-east-1   t3.micro   running   10.32.147.206    3s
i-bd2c658825f7ab669   us-east-1   t3.micro   running   10.140.149.192   3s
```

Asking for neither is a third thing, not the same as `-A`: it means whichever
namespace the provider calls its default, and only the provider can answer that
— the aws provider's default region comes from the same AWS configuration it
authenticates with.

## Changing

The object you read is the object you apply. `get -o yaml` produces a manifest
that `apply` accepts unchanged, which is what makes the round trip safe to build
on:

```sh
whoctl get linux/user alice -o yaml > alice.yaml
$EDITOR alice.yaml
whoctl apply -f alice.yaml
```

```console
$ whoctl apply -f alice.yaml
linux/user/alice configured
```

`edit` is those three steps in one, and `--dry-run` prints what would happen
without touching anything:

```sh
whoctl edit linux/user alice
whoctl apply -f team.yaml --dry-run
```

A manifest can hold many objects, so a machine's shape is a file somebody
reviews:

```yaml
apiVersion: linux.whoctl.io/v1alpha1
kind: Group
metadata:
  name: developers
spec:
  gid: 1500
---
apiVersion: linux.whoctl.io/v1alpha1
kind: User
metadata:
  name: alice
spec:
  shell: /bin/bash
  groups:
    - developers
```

## Removing

```console
$ whoctl delete linux/user alice --cascade
linux/user/alice deleted
```

Deletion is the one verb whose mistake cannot be undone by running the command
again. It takes names or a manifest, never a wildcard, and `--cascade` is what
says the home directory goes too — without it the account is removed and the
files stay.

## Serving it to kubectl, k9s and Lens

`whoctl serve` answers Kubernetes' own API, so somebody holding nothing but
kubectl, k9s or Lens can point at it and work. Nothing is translated on the way
out: a kind is already a group, a version and a kind, `metadata` already carries
what a client reads, and the columns a provider published are what the server
prints.

Each context in the configuration is a cluster to whoever connects, served under
its own path. That is how one server offers several: the provider is the same
binary each time and cannot tell which of them it is serving.

```yaml
apiVersion: whoctl.io/v1alpha1
kind: ServerConfig
listen: 127.0.0.1:8080

contexts:
  - name: machine
    provider: linux

  # One AWS account. The environment is the whole mechanism: the provider reads
  # its own vendor's credential chain from there, exactly as it would if a
  # person had exported the same variables in a shell.
  - name: prod
    provider: aws
    env:
      AWS_REGION: us-east-1
      AWS_PROFILE: prod
```

`--check` says what it would serve without listening:

```console
$ whoctl serve -f whoctl.yaml --check
would listen on 127.0.0.1:8080

context "machine" — 15 kinds at /contexts/machine
  groups                       linux.whoctl.io/v1alpha1
  users                        linux.whoctl.io/v1alpha1
  ...
```

and then a client points at one context:

```sh
kubectl --server http://127.0.0.1:8080/contexts/machine api-resources
```

### It listens on loopback and refuses anything else

There is no authentication yet. A server holding a configured AWS profile acts
for anybody who can reach the port, so a non-loopback address is refused rather
than warned about — the check runs before it serves anything, and there is no
flag to override it. That changes when authentication exists, and not before.

### What a client sees

The definitions under `apiextensions.k8s.io` are synthesized: their content is
read from what the provider published, so it is accurate, but nothing installed
them and every one says so in an annotation. A client's tree of custom resources
is built from these.

`pods` is a shim, and off unless a context asks for it. k9s opens on a pod view,
so a context may nominate a kind to answer it — `pods: processes.linux.whoctl.io`
— and what that costs is real: a client that finds `pods` assumes `v1/Pod`, so
`/log`, exec and `kubectl get pods -o yaml` all mean something they do not. It
is off by default for that reason.

## What else is there

`whoctl resources` lists every kind the installed providers serve, with the
short names and what each one can do.

`whoctl providers` answers "which binaries am I actually running", which is a
harder question than it sounds once a provider is being built in a checkout
rather than installed:

```console
$ whoctl providers
NAME      VERSION   PROTOCOL   BUILT   PATH
coredns   dev       2          1h      /home/dev/whoctl/bin/whoctl-provider-coredns
linux     0.1.0     2          6d      /home/dev/.whoctl/providers/whoctl/linux/0.1.0/whoctl-provider-linux
steam     -         -          1y      /usr/local/bin/whoctl-provider-steam

steam did not start and was skipped: provider "steam" speaks protocol 1, whoctl speaks 2
```

A provider that would not start is still listed, because leaving it out makes a
skipped provider look like one nobody installed. `whoctl version` reports the
protocol whoctl speaks, which is the other half of that answer.

Each provider documents its own kinds — **[browse them](providers/index.html)**.

## Writing a provider

A provider is a separate program that speaks newline-delimited JSON over stdio,
so it can be written in any language. whoctl does not know what any of them
manage: it discovers binaries, speaks the protocol, and renders what comes back.

[`whoctl-sdk-go`](https://github.com/whoctl/whoctl-sdk-go) makes the Go case a
matter of implementing one interface, and its README is the guide.
