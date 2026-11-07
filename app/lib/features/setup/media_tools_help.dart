import 'dart:io';

import 'package:flutter/material.dart';

import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';

/// Installation is explicit: this dialog never runs a package manager.
Future<void> showMediaToolsHelp(BuildContext context, {bool pdf = false}) =>
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.setupInstallHelp),
        content: SizedBox(
          width: 520,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(l10n.setupInstallIntro),
                const SizedBox(height: Space.md),
                Text(
                  Platform.isMacOS
                      ? (pdf
                            ? l10n.setupPdfHelperMissing
                            : l10n.setupMacInstall)
                      : Platform.isWindows
                      ? l10n.setupWindowsInstall
                      : Platform.isLinux
                      ? l10n.setupLinuxInstall
                      : l10n.setupOtherInstall,
                ),
                const SizedBox(height: Space.md),
                if (Platform.isMacOS && !pdf)
                  const SelectableText('brew install ffmpeg'),
                if (Platform.isLinux)
                  SelectableText(
                    pdf
                        ? 'sudo apt install poppler-utils'
                        : 'sudo apt install ffmpeg',
                  ),
                const SizedBox(height: Space.md),
                SelectableText(
                  pdf && Platform.isMacOS
                      ? 'https://github.com/WALKERKILLER/Coursive/releases'
                      : 'https://ffmpeg.org/download.html\nhttps://poppler.freedesktop.org/\nhttps://brew.sh/',
                ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: Text(l10n.commonClose),
          ),
        ],
      ),
    );
