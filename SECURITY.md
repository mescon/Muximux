# Security Policy

## Supported Versions

Security fixes are released for the latest 3.x version only. Muximux 1.x and
2.x (the PHP versions) are no longer maintained and will not receive fixes.

Upgrading is a matter of replacing the binary or pulling the new container
image; your `config.yaml` is carried forward.

## Reporting a Vulnerability

**Please do not open a public issue, pull request, or discussion for a
security problem.** Public reports disclose the flaw to everyone running
Muximux before a fix exists.

Report it privately through GitHub:

1. Go to the [Security tab](https://github.com/mescon/Muximux/security).
2. Choose **Report a vulnerability**.
3. Describe the issue, the version you tested, and how to reproduce it.

This opens a private advisory visible only to you and the maintainer. If you
prefer, you can request a CVE through the same advisory once a fix is ready.

### What to include

The more of this you can provide, the faster it can be confirmed:

- The version (`muximux -version`) and how you run it (binary, container).
- Relevant `config.yaml` settings with secrets removed, particularly anything
  under `auth`, `server` and any proxied app definitions.
- A request sequence, script or proof of concept that reproduces it.
- What an attacker gains: unauthenticated access, another user's session, a
  request reaching a backend it should not, and so on.

### What to expect

This is a single-maintainer project, so response times are best effort rather
than contractual. In practice you can expect an acknowledgement within a few
days, an assessment of whether the report is confirmed, and notice when a fix
ships. You will be credited in the release notes and the advisory unless you
ask not to be.

Please give a reasonable window for a fix before disclosing publicly.

## Scope

In scope, roughly in order of severity:

- Authentication or authorization bypass, including reaching a proxied backend
  without a valid session, or escaping the paths an `auth_bypass` rule is meant
  to permit.
- Anything that lets one user act as another, or a non-admin perform an
  admin-only action such as container control.
- Server-side request forgery through app or gateway site definitions.
- Injection into the content rewriter that results in script execution in the
  dashboard origin.
- Exposure of secrets from `config.yaml`, session cookies or the Docker socket.
- Path traversal in any handler that reads from disk (themes, icons, config).

Out of scope:

- Vulnerabilities in the applications you proxy. Muximux embeds them; it does
  not make them safe.
- Findings that require an attacker who already has admin access to the
  dashboard, or write access to `config.yaml` or the host.
- Exposing Muximux directly to the internet without a session secret,
  HTTPS or authentication configured, contrary to the deployment docs.
- Missing hardening headers on endpoints that serve no content.
- Automated scanner output with no demonstrated impact.

## Verifying a Download

Every release artifact is published with a signed SLSA provenance attestation
and an SPDX SBOM, so you can confirm a binary or image came from this
repository's release workflow rather than trusting the release page:

```bash
gh attestation verify muximux-linux-amd64 --repo mescon/Muximux \
  --signer-workflow mescon/Muximux/.github/workflows/release.yml
```

Naming the workflow matters: `--repo` on its own only asserts that some
workflow in the repository signed the artifact.

See [Verifying a Download](https://github.com/mescon/Muximux/wiki/installation#verifying-a-download)
for the container image equivalent and checksum verification.

## Security Posture

The controls built into Muximux, the standards they are measured against, and
the deployment guidance are documented in
[the security wiki page](https://github.com/mescon/Muximux/wiki/security).

Continuous checks on every change: CodeQL, golangci-lint including gosec,
govulncheck, Snyk, SonarCloud, Dependabot, and fuzz targets over the proxy's
parsing paths.
