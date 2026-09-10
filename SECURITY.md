# Security Policy

## Reporting a vulnerability

Please do not open a public issue for security problems.

Use GitHub's private vulnerability reporting: open the **Security** tab on the repository and choose
**Report a vulnerability**. That creates a private advisory that only the maintainer can see.

Include what you can: the affected version (`aat --version`), steps to reproduce, and what an
attacker could do with it.

## Supported versions

Only the latest minor release line receives security fixes. If you are on an older release, upgrade
first and check whether the problem still reproduces.

## Response

AAT has a single maintainer, so response is best effort and there is no SLA. Reports are read and
acknowledged as they come in, and a fix or workaround follows as soon as practical. Reporters are
credited in the advisory and release notes unless they ask otherwise.

## Things to know when running AAT

- **Run archives redact auth headers.** `Authorization`, `Proxy-Authorization`, `X-API-Key`,
  `X-Auth-Token`, `Cookie`, and `Set-Cookie` values are replaced with `[REDACTED]` before an archive
  or `.aar` export is written, and step inputs and resolved values are scrubbed of known secret
  values. Request and response *bodies* are stored as-is, so review them before sharing an archive.
- **`--dump-state` files contain live credentials.** The live-state export written by
  `aat run plan --dump-state FILE` includes the resolved base URL, live auth headers, and step outputs
  so an external harness can pick up where a run left off. The file is written with mode `0600`.
  Never commit these files, attach them to issues, or leave them in a shared location.
- **`aat mcp serve --http` has no authentication.** The HTTP transport listens on all interfaces
  (`:8080` by default; there is no `--host` flag) and accepts any request. Keep it on a machine
  that is not reachable from untrusted networks, or put it behind a reverse proxy that handles
  authentication and TLS. The default stdio transport does not have this exposure.
- **`aat web` is a local tool.** The web UI also listens on all interfaces and serves your run
  archives to anyone who can reach the port. Do not expose it to untrusted networks.
