# Device preparation and release tools

Artifact-Version: 1

## Executive summary

A tester could stream lectures but transcription failed because the FFmpeg executable was unavailable. The setup flow introduced courses without making optional dependencies visible, and a development machine's installed tools masked a minimal desktop-launch environment. The same audit found that macOS PDF imports required a helper absent from the release staging steps. Feature readiness must follow the actual installed backend and artifact, not infer success from another feature working.

## Summary

Playback uses the media player while transcription invokes a separate FFmpeg executable. Local video import additionally uses ffprobe. The PDF backend is PDFKit on macOS and Poppler in the Android Linux workspace; a generic Poppler installation hint cannot repair a missing macOS helper. Findings are tracked in [issue 15](https://github.com/WALKERKILLER/Coursive/issues/15) and [issue 16](https://github.com/WALKERKILLER/Coursive/issues/16).

## Timeline

- 2026-11-08: the user supplied a fresh-install transcription failure screenshot.
- Source inspection traced desktop process PATH, optional component diagnostics, and native PDF helper staging.
- Regression checks exercised partial tool installations, unavailable assistant endpoints, rechecking after installation and platform-specific PDF requirements.
- Local macOS and Android builds exercised the preparation entry; Android installed FFmpeg and Poppler through the existing workspace installers. The macOS universal helper rendered a synthetic PDF to an image and text. The hosted release workflow itself was not run during this audit.

## Root cause

The application treated playback, transcription, PDF import and AI setup as separate implementations without a shared user-visible preparation entry. Diagnostics exposed only some dependencies and let one failed request skip later checks. Desktop startup inherited the graphical session PATH, which can omit package-manager directories. Release staging carried the daemon and agent but did not carry the PDF helper invoked by the daemon; local manual installation concealed the omission.

## Guardrails

- [Diagnostics regression](../../app/test/diagnostics_test.dart) checks independent failures, partial installations, refresh and PDF backend-specific guidance; unknown results must not retain stale success.
- [Daemon tool detection test](../../daemon/internal/server/media_tools_test.go) distinguishes missing and partially installed executables, while [desktop PATH regression](../../app/test/daemon_path_test.dart) preserves explicit tool priority and includes standard package-manager locations.
- [macOS release workflow](../../.github/workflows/release.yml) compiles both PDF helper architectures, verifies them, copies the helper beside the daemon and checks executable presence before signing.
- [Feature reference](../features.md) separates executable presence from decoding, model and network validation. Windows PDF compatibility remains explicitly unsupported rather than inferred from installed tools.
