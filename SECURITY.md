# Security policy

## Supported versions

Security support covers the latest release and the current `main` branch.
Update to the most recent version before reporting an issue when practical.

## Reporting a vulnerability

Email `git@sunsung.fun` with the subject `SECURITY: <short summary>`.

Do not open a public issue, discussion, or pull request for a suspected
vulnerability.

Include the affected version or commit, operating system, controller model,
relevant configuration, the affected command or package, impact, and clear
reproduction steps. Remove credentials, network addresses, and personal data
from logs or packet captures.

We will acknowledge the report within 72 hours and provide an initial
assessment or mitigation plan within 14 days. We aim to fix or mitigate
confirmed issues within 90 days; complex issues can take longer.

## Disclosure

Please keep the report private until a fix or mitigation is available. We will
coordinate public disclosure and credit you unless you ask not to be named.

## Out of scope

The following are usually out of scope unless they demonstrate a concrete,
exploitable weakness in this project:

- Social engineering or physical attacks.
- Pure denial-of-service, spam, or resource-exhaustion reports without an
  actionable fix.
- Automated scanner output without confirmed exploitability.
- Issues affecting only unsupported versions or platforms.
- Vulnerabilities in third-party dependencies that should be reported upstream.

## Safe harbor

If you follow this policy and act in good faith, we will not pursue legal
action or intentionally block your research. Test only what is necessary to
demonstrate the issue, do not access data that is not yours, and stop if your
testing could harm a controller, network, or user.

## Security updates

Security fixes are released as patch versions when applicable and noted in the
GitHub release notes.
