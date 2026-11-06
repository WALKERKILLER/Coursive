# Security policy

## Reporting a vulnerability

Report vulnerabilities privately through GitHub: open the repository's **Security** tab and choose **Report a vulnerability**. Do not open a public issue, pull request or discussion for a suspected vulnerability.

Include what an attacker can do, the affected component and version, and the steps or proof of concept that show it. You will receive an acknowledgement within 7 days and an assessment within 30 days. Please allow a fix to ship before any public disclosure.

Vulnerabilities in a third-party plugin go to that plugin's author; vulnerabilities in the plugin index, the mirror list or an official plugin go to [coursive-plugins](https://github.com/WALKERKILLER/coursive-plugins/security).

## Supported versions

Coursive is pre-release software. Only the latest release or pre-release receives fixes.

## Scope

In scope:

- The local daemon's API: authentication of local clients, path handling, and anything that exposes library files or sealed credentials.
- The credential vault and every place a secret could be logged, persisted unsealed, or returned.
- Plugin installation: index signature checks, artifact hashes, consent and permission enforcement, and mirror handling.
- The assistant's write guards and approval mode.

Out of scope: vulnerabilities that require an attacker who already controls the user's account on the device, and issues in external course platforms or model providers themselves.
