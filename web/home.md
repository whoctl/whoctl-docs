## Install

```sh
curl -fsSL https://raw.githubusercontent.com/whoctl/whoctl/main/install.sh | sh
```

On Windows, take `whoctl-windows-amd64.exe` from the
[releases page](https://github.com/whoctl/whoctl/releases/latest).

## Two ways to run it, and the provider cannot tell them apart

**On your own machine.** whoctl starts the providers itself, and they act as
whoever you are — your `~/.aws`, your SSO session, your uid. Nothing is
configured and nothing is shared.

```console
$ whoctl get linux/users
$ AWS_PROFILE=prod whoctl get aws/route53/hostedzones
```

**As a server.** One machine holds the configuration — ten AWS accounts, a
CoreDNS per edge box — and hands each out as a cluster. whoctl answers
Kubernetes' own API, so the clients are ones people already have:

```console
$ whoctl serve -f whoctl.yaml
whoctl serving on http://127.0.0.1:6443
  context machine        http://127.0.0.1:6443/contexts/machine
  context prod           http://127.0.0.1:6443/contexts/prod

$ kubectl --server http://127.0.0.1:6443/contexts/machine get users
NAME     UID     GID     GROUP    SHELL
root     0       0       root     /bin/bash
alice    1000    1000    alice    /bin/sh
```

**whoctl speaks to it too.** `--server` reads through a server instead of
running providers here, and every verb, flag and output format is the one you
already type:

```console
$ whoctl get linux/users --server http://127.0.0.1:6443/contexts/machine
$ export WHOCTL_SERVER=http://127.0.0.1:6443/contexts/prod
$ whoctl get aws/route53/hostedzones
```

kubectl takes the address on the command line. k9s and Lens want a kubeconfig,
which is the same address in the shape they read:

```yaml
apiVersion: v1
kind: Config
clusters:
  - name: whoctl-prod
    cluster:
      server: http://127.0.0.1:6443/contexts/prod
contexts:
  - name: whoctl-prod
    context: {cluster: whoctl-prod, user: whoctl}
users:
  - name: whoctl
    user: {}
current-context: whoctl-prod
```

A context is a cluster and nothing else has to be true for that: point
`KUBECONFIG` at the file and every one of them opens.

The provider is the same binary in both, and **cannot tell which of the two
started it**. That is the whole design rather than a coincidence: the only thing
that differs is who writes the process's environment — your shell, or the
server, per context. There is no credential field on a manifest, no flag on
whoctl, and no branch inside a provider for it.

### What a server can reach, per provider

"Remote" is not one answer, because a provider reaches whatever it reaches:

| Provider | A context can serve |
| --- | --- |
| `aws` | any account its credentials reach. Genuinely remote — a server holds ten and the machine it runs on is irrelevant. |
| `coredns` | a tree the server can open: a mounted image, an rsync'd copy, a local install. |
| `steam` | the same — an installation on a path it can read. |
| `linux` | **the machine the server itself runs on.** It reads `/etc` and shells out to `useradd`, and both happen where the process is. |

That last row is the one to know before building on it: ten machines are ten
servers today. A remote mode for `linux` — reaching another host over ssh — is a
real design and is not built.

### What whoctl gives you that kubectl does not

kubectl works, completely, and for a Kubernetes shop that is the point. What the
other binary is for:

| | kubectl | whoctl |
| --- | --- | --- |
| Addressing | `kubectl get hostedzones.route53.aws.whoctl.io` | `whoctl get aws/route53/hostedzones` — the group as a path, and `aws/hz` when nothing collides |
| Which provider | not a concept: a kind is a group | in every name, so `linux/users` and `aws/users` never get confused for each other |
| `describe` | the object's fields | the fields **and what the provider says each one means** |
| Off a server | needs a cluster | the same command runs the providers locally, with no server at all |
| Setup | a kubeconfig | an address, or none |

The addressing is the one that shows up on every line. A kind's group is a path
read backwards — `route53.aws.whoctl.io` is `aws/route53` — and whoctl reads it
that way against a server exactly as it does locally, because the server says
which provider serves each kind in an annotation on the definition.

The documentation is the one that shows up when something is unfamiliar.
`whoctl describe` against a server shows the provider's own doc text for every
field, because it travelled in the definition's `description` — the same place a
CRD keeps exactly this.

### One thing that is not there yet

**A server listens on loopback and refuses anything else.** There is no
authentication, so anybody who reached the port would act with every credential
the contexts hold. The refusal happens before it serves anything and there is no
flag to override it. That changes when authentication exists, and not before.

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

The configuration behind the second of the two ways above.

Nothing is translated on the way out: a kind is already a group, a version and
a kind, `metadata` already carries what a client reads, and the columns a
provider published are what the server prints. Each provider's own page says
what its context needs — which environment variables, what a namespace means
there, and whether anything it serves can answer the pod view.

Each context in the configuration is a cluster to whoever connects, served under
its own path. That is how one server offers several: the provider is the same
binary each time and cannot tell which of them it is serving.

```yaml
apiVersion: whoctl.io/v1alpha1
kind: ServerConfig
listen: 127.0.0.1:6443
# Kubernetes' own port, so pointing a client here is the same gesture as
# pointing it at a cluster — and so it is not 8080, which everything wants.

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
would listen on 127.0.0.1:6443

context "machine" — 15 kinds at /contexts/machine
  groups                       linux.whoctl.io/v1alpha1
  users                        linux.whoctl.io/v1alpha1
  ...
```

and then a client points at one context:

```sh
kubectl --server http://127.0.0.1:6443/contexts/machine api-resources
```

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
