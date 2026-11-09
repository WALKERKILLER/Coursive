import 'package:flutter/material.dart';

import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';

/// Platform admission of a plugin Web UI page
/// (docs/specs/0032-ui-extension-protocol.md, Isolated Web UI). The daemon owns
/// the record; the app only explains it.
enum PluginWebAdmission {
  supported('supported'),
  needsReview('needs-review'),
  unsupported('unsupported');

  const PluginWebAdmission(this.code);

  /// The stable code the daemon publishes.
  final String code;

  /// An unknown or missing code is treated as "still under review", never as
  /// supported: a page is only rendered once the host says so.
  static PluginWebAdmission fromCode(Object? code) => switch ('$code') {
    'supported' => PluginWebAdmission.supported,
    'unsupported' => PluginWebAdmission.unsupported,
    _ => PluginWebAdmission.needsReview,
  };
}

/// Why a Web UI page is not rendered here. The codes match the daemon's
/// fallback reasons in `daemon/internal/pluginfabric/webui.go`.
enum PluginWebFallbackReason {
  platformUnsupported('platform-unsupported'),
  platformNeedsReview('platform-needs-review'),
  rendererMissing('renderer-missing'),
  reviewPending('review-pending'),
  pluginInactive('plugin-inactive'),
  resourcesMissing('resources-missing');

  const PluginWebFallbackReason(this.code);

  final String code;

  /// An unknown or missing reason is read as a missing renderer, never as a
  /// page that may render.
  static PluginWebFallbackReason fromCode(Object? code) => switch ('$code') {
    'platform-unsupported' => PluginWebFallbackReason.platformUnsupported,
    'platform-needs-review' => PluginWebFallbackReason.platformNeedsReview,
    'review-pending' => PluginWebFallbackReason.reviewPending,
    'plugin-inactive' => PluginWebFallbackReason.pluginInactive,
    'resources-missing' => PluginWebFallbackReason.resourcesMissing,
    _ => PluginWebFallbackReason.rendererMissing,
  };

  String get message => switch (this) {
    PluginWebFallbackReason.platformUnsupported =>
      l10n.pluginWebReasonPlatformUnsupported,
    PluginWebFallbackReason.platformNeedsReview =>
      l10n.pluginWebReasonNeedsReview,
    PluginWebFallbackReason.rendererMissing =>
      l10n.pluginWebReasonRendererMissing,
    PluginWebFallbackReason.reviewPending => l10n.pluginWebReasonReviewPending,
    PluginWebFallbackReason.pluginInactive =>
      l10n.pluginWebReasonPluginInactive,
    PluginWebFallbackReason.resourcesMissing =>
      l10n.pluginWebReasonResourcesMissing,
  };
}

/// The native fallback for a plugin page whose declared renderer is `web-ui`.
///
/// It explains why the page cannot render and keeps the plugin's L1/L2 surfaces
/// — native pages, UI slots and actions — usable instead of showing a blank
/// screen. A platform without a stable WebView degrades one page, not the app.
class PluginWebFallbackPage extends StatelessWidget {
  const PluginWebFallbackPage({
    super.key,
    required this.pluginTitle,
    this.admission = PluginWebAdmission.needsReview,
    this.reason = PluginWebFallbackReason.rendererMissing,
    this.detail,
    this.onOpenNative,
    this.showAppBar = true,
  });

  /// The page title the plugin contributed.
  final String pluginTitle;
  final PluginWebAdmission admission;
  final PluginWebFallbackReason reason;

  /// An optional host-supplied explanation, such as a missing resource name.
  final String? detail;

  /// Opens the plugin's native page when it contributes one.
  final VoidCallback? onOpenNative;
  final bool showAppBar;

  @override
  Widget build(BuildContext context) {
    final body = _body(context);
    if (!showAppBar) return body;
    return Scaffold(
      appBar: AppBar(title: Text(pluginTitle)),
      body: body,
    );
  }

  Widget _body(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    return ListView(
      key: const ValueKey('plugin-web-fallback'),
      padding: const EdgeInsets.fromLTRB(
        Space.xl,
        Space.xxl,
        Space.xl,
        Space.xxl,
      ),
      children: [
        Center(
          child: Container(
            width: 72,
            height: 72,
            decoration: BoxDecoration(
              color: cs.secondaryContainer,
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.web_asset_off_outlined,
              size: 32,
              color: cs.onSecondaryContainer,
            ),
          ),
        ),
        const SizedBox(height: Space.xl),
        Text(
          l10n.pluginWebUnavailableTitle,
          style: tt.titleLarge,
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: Space.sm),
        Text(
          reason.message,
          style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant),
          textAlign: TextAlign.center,
        ),
        if (detail != null && detail!.isNotEmpty) ...[
          const SizedBox(height: Space.xs),
          Text(
            detail!,
            style: tt.bodySmall?.copyWith(color: cs.onSurfaceVariant),
            textAlign: TextAlign.center,
          ),
        ],
        const SizedBox(height: Space.xl),
        Center(child: _admissionChip(context)),
        const SizedBox(height: Space.xl),
        Container(
          padding: const EdgeInsets.all(Space.lg),
          decoration: BoxDecoration(
            color: cs.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(Corners.md),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(
                    Icons.extension_outlined,
                    size: 18,
                    color: cs.onSurfaceVariant,
                  ),
                  const SizedBox(width: Space.sm),
                  Text(l10n.pluginWebNativeKeptTitle, style: tt.titleSmall),
                ],
              ),
              const SizedBox(height: Space.sm),
              Text(
                l10n.pluginWebNativeKept,
                style: tt.bodySmall?.copyWith(color: cs.onSurfaceVariant),
              ),
            ],
          ),
        ),
        const SizedBox(height: Space.xl),
        if (onOpenNative != null)
          FilledButton.tonalIcon(
            onPressed: onOpenNative,
            icon: const Icon(Icons.open_in_new_rounded),
            label: Text(l10n.pluginWebOpenNative),
          ),
        const SizedBox(height: Space.sm),
        TextButton(
          onPressed: () => Navigator.of(context).maybePop(),
          child: Text(l10n.commonBack),
        ),
      ],
    );
  }

  Widget _admissionChip(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final label = switch (admission) {
      PluginWebAdmission.supported => l10n.pluginWebAdmissionSupported,
      PluginWebAdmission.needsReview => l10n.pluginWebAdmissionNeedsReview,
      PluginWebAdmission.unsupported => l10n.pluginWebAdmissionUnsupported,
    };
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: Space.md,
        vertical: Space.xs,
      ),
      decoration: BoxDecoration(
        border: Border.all(color: cs.outlineVariant),
        borderRadius: BorderRadius.circular(Corners.full),
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.labelMedium
            ?.copyWith(color: cs.onSurfaceVariant),
      ),
    );
  }
}
