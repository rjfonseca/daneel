# Security Policy

## Supported Versions

We release patches and security fixes for the latest active release versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x (once released) | Latest minor release receives fixes |
| 0.x / release candidates | Development only; upgrade to the latest candidate |

## Reporting a Vulnerability

We take the security of **Daneel** very seriously. If you believe you have found a security vulnerability in Daneel (such as data leakage, insecure file operations, or memory safety issues), please report it responsibly.

### Responsible Disclosure Process

1. **Do not create a public GitHub issue** for sensitive vulnerabilities.
2. Please submit a report privately via **GitHub Security Advisories** on the repository page under `Security > Advisories > Report a vulnerability`.
3. Include as much detail as possible:
   - A description of the vulnerability.
   - Steps to reproduce the issue (proof of concept script or minimal reproduction steps).
   - Potential impact and affected versions.

### Our Commitment

* We will acknowledge receipt of your report within **48 hours**.
* We will provide a timeline for assessing and addressing the vulnerability.
* We will credit security researchers who adhere to responsible disclosure in the release notes.

## Local trust boundary

The Web server binds to `127.0.0.1` and rejects unexpected Host and Origin
headers. Do not expose it through a public reverse proxy or port forward: remote
authentication is not provided. Processes running as the same operating-system
user can invoke local APIs and are trusted. MCP access grants the connected agent
access to the selected vault and to the operations exposed by its tools.

Requested Web and MCP notes must stay inside the vault, including symlink
resolution. Export scanning is a defense in depth check, not a guarantee that all
possible secrets or personal information can be recognized.

Before release, maintainers must enable private vulnerability reporting in the
recreated repository so the reporting channel above is available.
