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

## A resource is always `provider/resource`

The prefix is not optional:

```console
$ whoctl get users
error: resource "users" needs its provider: try linux/users
```

`whoctl get users` does not say *whose* users, and once two providers serve a
kind of the same name the guess whoctl would have to make is the kind of mistake
that deletes the wrong account. Within that, the plural, the singular and any
short name all work — `linux/users`, `linux/user`, `linux/usr`.

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

## What else is there

`whoctl resources` lists every kind the installed providers serve, with the
short names and what each one can do. `whoctl providers` lists what this machine
has and at which version.

Each provider documents its own kinds — **[browse them](providers/index.html)**.

## Writing a provider

A provider is a separate program that speaks newline-delimited JSON over stdio,
so it can be written in any language. whoctl does not know what any of them
manage: it discovers binaries, speaks the protocol, and renders what comes back.

[`whoctl-sdk-go`](https://github.com/whoctl/whoctl-sdk-go) makes the Go case a
matter of implementing one interface, and its README is the guide.
