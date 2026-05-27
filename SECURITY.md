# Security Policy

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Use GitHub's [Private Vulnerability Reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability) instead:

1. Go to the [Security tab](../../security) of this repository.
2. Click **"Report a vulnerability"**.
3. Fill in the form — include reproduction steps and any proposed fix.

You will be notified through GitHub when the report is acknowledged. Expect a first response within **7 days**.

## Supported versions

Only `main` is supported. There are no released versions yet.

## Scope

In scope:
- Authentication and authorization bugs (login, JWT, refresh-token rotation, lockout bypass).
- Injection issues (SQL, command, header).
- Secret leakage in logs or responses.
- Issues that allow one user to access another user's session, tokens, or data.

Out of scope:
- Findings that require pre-existing access to the home box, the deploy SSH key, or repository secrets.
- Misconfigurations in deployments that are not this repository's defaults.
- Denial of service against a single user-controlled home instance.
