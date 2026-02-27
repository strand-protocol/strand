# Security Policy

The Strand Protocol team takes security seriously. We appreciate your efforts to responsibly disclose your findings, and will make every effort to acknowledge your contributions.

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

As new major or minor versions are released, security support for older versions will be documented here.

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to:

**security@strandprotocol.com**

Please include the following information in your report:

- **Description** of the vulnerability
- **Steps to reproduce** the issue, including any proof-of-concept code
- **Impact assessment** — what an attacker could achieve by exploiting this vulnerability
- **Affected module(s)** — which Strand Protocol module(s) are affected (StrandLink, StrandRoute, StrandStream, StrandTrust, StrandAPI, StrandCtl, Strand Cloud)
- **Affected version(s)** — which version(s) you tested against
- **Your contact information** so we can follow up

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your report within **48 hours**.
- **Triage**: We will confirm the vulnerability and determine its severity within **5 business days**.
- **Fix timeline**: We aim to release patches for critical vulnerabilities within **14 days** of confirmation. Non-critical issues may take longer depending on complexity.
- **Notification**: You will be kept informed of our progress throughout the process.

## Disclosure Policy

We follow a **coordinated disclosure** process:

1. The reporter submits the vulnerability privately to our security team.
2. Our team confirms, triages, and develops a fix.
3. We prepare a security advisory and coordinate a release date with the reporter.
4. The fix is released and the advisory is published simultaneously.
5. The reporter is credited in the advisory (unless they prefer to remain anonymous).

We ask that you:

- Allow us reasonable time to address the issue before making any public disclosure.
- Make a good faith effort to avoid privacy violations, destruction of data, and interruption or degradation of our services.
- Do not access or modify data belonging to other users.

## Bug Bounty

We are planning a formal bug bounty program for future releases. In the meantime, we will publicly credit security researchers who report valid vulnerabilities (with their permission) and are happy to discuss other forms of recognition on a case-by-case basis.

## Scope

The following areas are in scope for security reports:

- **StrandTrust (L4)**: Model identity, cryptographic operations, certificate handling, key management
- **StrandStream (L3)**: Transport security, connection handling, data integrity
- **StrandRoute (L2)**: Routing table poisoning, semantic address spoofing
- **StrandLink (L1)**: Frame injection, buffer overflows, memory safety issues
- **StrandAPI (L5)**: Authentication bypass, authorization flaws, injection attacks
- **StrandCtl**: Privilege escalation, credential handling
- **Strand Cloud**: Control plane security, multi-tenancy isolation, API security

## Security-Related Configuration

For guidance on securely configuring Strand Protocol deployments, refer to the [Security Hardening Guide](docs/security-hardening.md) (coming soon).

## Contact

- **Email**: security@strandprotocol.com
- **PGP Key**: Available upon request

Thank you for helping keep Strand Protocol and its users safe.
