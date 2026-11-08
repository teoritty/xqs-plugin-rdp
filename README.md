# xqs-plugin-rdp

An RDP client for [xQuakShell](https://github.com/teoritty/xQuakShell). It adds
an `rdp` connection protocol and renders the remote desktop in an embedded
[IronRDP](https://github.com/Devolutions/IronRDP) viewer inside the session tab.

> **Status: proof of concept.** The plugin works — it connects, authenticates
> and gives you a usable remote desktop — but only the essentials are there:
> **the picture, the keyboard and the mouse.** Everything else (clipboard,
> audio, drive redirection, multi-monitor, …) is not implemented yet. Expect
> rough edges and breaking changes between pre-releases.

## What works

- Connects to an RDP server (default port **3389**) with **NLA / CredSSP**.
- TLS and CredSSP terminate **host-side, in the plugin process** — the password
  never reaches the browser. The embedded WASM viewer only ever sees an
  already-authenticated stream.
- **Screen output, keyboard and mouse**, fitted to the session tab.
- Server certificate trust, confirmed once per server (see below).

That is the whole feature set of this PoC. Anything not on this list should be
assumed missing.

## Requirements

- xQuakShell with plugin API `1.0.0`.
- A reachable RDP server supporting **NLA (CredSSP over TLS)** with **NTLM**.

## Install

Point xQuakShell's plugin manager at this repository (**Add GitHub
repository**), or download the `.xqsp` bundle for your platform from the
[Releases](https://github.com/teoritty/xqs-plugin-rdp/releases) page and install
it via **Settings → Plugins → Install bundle…**. Releases are published as
pre-releases while the plugin is a PoC.

## Not implemented

Beyond the PoC's picture, keyboard and mouse, nothing else is wired up. Some of
it is deliberately excluded, each for a stated reason:

| Not included | Why |
|---|---|
| Clipboard | Blocked outside this plugin: the embed iframe is cross-origin to the app and carries no `allow=` attribute, so the Clipboard API is denied by Permissions Policy. |
| Read-only mode | Would require parsing client RDP PDUs, which breaks the plugin's core invariant of not decoding the stream after authentication. |
| Kerberos | NTLM only. In domains with NTLM disabled this plugin will not connect. |
| Audio, drive and printer redirection | Out of scope; drive redirection in particular hands the remote machine access to local files. |
| RD Gateway, Hyper-V VMConnect, multi-monitor | Out of scope. |

## Third-party code

The viewer is [IronRDP](https://github.com/Devolutions/IronRDP)'s WebAssembly
web client, vendored under `ui/vendor/ironrdp/`:

| Package | Version |
|---|---|
| `@devolutions/iron-remote-desktop` | 0.11.0 |
| `@devolutions/iron-remote-desktop-rdp` | 0.7.0 |

Both are `MIT OR Apache-2.0`, which is compatible with this plugin's GPL-3.0;
the licence text is kept alongside the bundle. The RDP package is ~6.1 MB
because the WebAssembly module is embedded in it — that is the cost of not
writing an RDP stack from scratch.

It is vendored rather than downloaded during the build so that the release
signature covers exactly the bytes that were reviewed. `SHA256SUMS` in that
directory pins them; `VERSION` records where they came from and what to check
when updating.

The Go side depends on [`bodgit/ntlmssp`](https://github.com/bodgit/ntlmssp)
for NTLM with signing, sealing and channel bindings — the parts CredSSP needs.
CredSSP itself has no ready-made Go implementation and is written here.

## Building

    go test ./...
    go run ./cmd/xqs-rdp-pack

The second command cross-compiles for six platforms and writes an installable
`.xqsp` bundle per platform into `dist/`. Add `-only windows/amd64` to build
just one.

## Releases

Pushing a tag `vX.Y.Z` (pre-release suffixes such as `v1.0.0-pre.2` included)
builds and publishes a GitHub release through `.github/workflows/release.yml`.
It runs the same gates as a branch build (gofmt, vet, the race-tested suite),
checks that `plugin.json` and `xqsp.json` agree on `id` and `version` and that
the version matches the tag, then attaches the six `.xqsp` bundles and a
`SHA256SUMS` over them. To cut one, bump `version` in both manifests, commit, then:

    git tag v1.0.0-pre.2
    git push origin v1.0.0-pre.2

Bundles are **unsigned**. The host's signature is Ed25519 over a canonical JSON
envelope containing its own `plugin.Manifest` Go struct, so signing from outside
the core means reproducing that struct field for field — which would keep
working right up until the core changes it, and then fail silently rather than
loudly. An unsigned bundle installs with a visible "unsigned" notice, which is
the honest version of the same state.

## Server certificate trust

RDP servers are almost always self-signed, and CredSSP's channel binding is only
worth anything if you know whose key you are bound to. So before any credential
leaves this process, the plugin asks the core whether the server on the other end
is one you have already trusted.

The plugin sends the server's SubjectPublicKeyInfo and gets back yes or no. On
"no" the core shows you the fingerprint and asks; your answer is recorded there,
and the connection retries on its own. Nothing about that decision is stored in
the plugin or in the connection's fields — a plugin runs with `per-session`
isolation and has nowhere durable to keep it, and a field you fill in by hand is
a question asked in the wrong place.

Trust is remembered per address (`host:port`) within this plugin, the way
`known_hosts` remembers a host rather than a saved connection. Two connections
pointing at the same server share it. A key that later changes is a different
question with different wording, not a repeat of the first one.

The material is the SubjectPublicKeyInfo, not the whole certificate: a server may
reissue a certificate with the same key, and asking you to re-confirm that would
train you to confirm key changes without looking — the exact habit this protects
against.

Older versions of this plugin had a *Server key fingerprint* field on the
connection form. It is gone; nothing needs to be migrated, and connections that
still carry a value for it simply ignore it. This requires a core with the peer
trust service (`trust.verifyPeer`); on an older core the connection fails with a
capability error rather than proceeding unverified.

## License

GPL-3.0.
