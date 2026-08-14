# xqs-plugin-rdp

An RDP client for [xQuakShell](https://github.com/teoritty/xQuakShell). It adds
an `rdp` connection protocol and renders the remote desktop in an embedded
[IronRDP](https://github.com/Devolutions/IronRDP) viewer inside the session tab.

**Status: in development.** Nothing here works yet.

## What it will do

- Connect to an RDP server (default port **3389**) with **NLA / CredSSP**.
- Terminate TLS and CredSSP **host-side, in the plugin process** — the password
  never reaches the browser. The embedded WASM viewer only ever sees an
  already-authenticated stream.
- Screen, mouse and keyboard, fitted to the session tab.

## Requirements

- xQuakShell with plugin API `1.0.0`.
- A reachable RDP server supporting **NLA (CredSSP over TLS)** with **NTLM**.

## Scope of the first version

Deliberately excluded, each for a stated reason:

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

## Server certificate trust

RDP servers are almost always self-signed, and CredSSP's channel binding is only
worth anything if you know whose key you are bound to. The first connection to a
server therefore **fails**, showing the server's SPKI fingerprint; paste it into
the connection's *Server key fingerprint* field to trust that server. A
fingerprint that later changes fails loudly.

This is `StrictHostKeyChecking=yes` behaviour rather than trust-on-first-use,
because a plugin has nowhere durable to record what it has seen before.

## License

GPL-3.0.
