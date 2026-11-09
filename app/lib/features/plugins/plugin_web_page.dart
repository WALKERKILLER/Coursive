import 'dart:async';
import 'dart:convert';
import 'dart:io' show Platform;

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:webview_flutter/webview_flutter.dart';
import 'package:webview_flutter_android/webview_flutter_android.dart';

import '../../core/api.dart';
import '../../core/app_state.dart';
import '../../core/plugin_navigation.dart';
import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';
import 'plugin_actions.dart';
import 'plugin_contributions.dart';
import 'plugin_web_fallback.dart';

/// The Web UI (L3) renderer of a plugin page
/// (docs/specs/0032-ui-extension-protocol.md, Isolated Web UI).
///
/// The daemon owns the page: it decides the platform admission, builds the
/// document with its CSP and session nonce, answers every navigation and
/// verifies every message. The app only loads that document into the system
/// WebView, relays what the page's `CoursiveBridge` channel receives together
/// with the frame it recorded, asks before a write and before an external link,
/// and hands the answer back to the page. The page gets no token, no daemon URL
/// and no other channel.

/// What `POST /api/plugin-web/sessions` answered.
class PluginWebOpening {
  PluginWebOpening.fromJson(Map<String, dynamic> json)
    : admission = PluginWebAdmission.fromCode(
        _map(json['admission'])?['status'],
      ),
      preview = json['preview'] == true,
      fallback = json['fallback'] == null
          ? null
          : PluginWebFallbackReason.fromCode(_map(json['fallback'])?['reason']),
      detail = '${_map(json['fallback'])?['detail'] ?? ''}',
      sessionId = '${_map(json['session'])?['id'] ?? ''}',
      baseUrl = '${_map(json['session'])?['baseUrl'] ?? ''}',
      html = '${json['html'] ?? ''}';

  final PluginWebAdmission admission;
  final bool preview;

  /// Set when the page cannot render here; [detail] may explain it.
  final PluginWebFallbackReason? fallback;
  final String detail;
  final String sessionId, baseUrl, html;

  bool get renders =>
      fallback == null && sessionId.isNotEmpty && html.isNotEmpty;
}

/// The daemon's answer to a navigation the page asked for.
enum PluginWebNavigation { sameOrigin, external, deny }

/// The platform name of the admission matrix, or the OS name for one outside
/// it (which the daemon reports as unsupported).
String pluginWebPlatform() {
  if (kIsWeb) return 'web';
  return switch (Platform.operatingSystem) {
    'android' => 'android',
    'macos' => 'macos',
    final other => other,
  };
}

/// The app side of one page session: opening it, relaying messages and
/// navigation questions, and asking the user before a write is applied.
///
/// [call] is `Api.call`; [confirm] shows the proposal to the user and answers
/// whether to apply it.
class PluginWebSession {
  PluginWebSession({required this.call, required this.confirm});

  final ApiCall call;
  final Future<bool> Function(PluginProposal proposal) confirm;
  String _id = '';

  static const _base = '/api/plugin-web/sessions';

  Future<PluginWebOpening> open({
    required String pluginId,
    required String pageId,
    required String platform,
  }) async {
    final raw = await call('POST', _base, {
      'pluginId': pluginId,
      'pageId': pageId,
      'platform': platform,
    });
    final opening = PluginWebOpening.fromJson(_map(raw) ?? const {});
    _id = opening.sessionId;
    return opening;
  }

  /// Relays one raw channel message with the frame the renderer recorded and
  /// returns the JSON answer for the page. A write proposal is shown to the
  /// user first, and the page receives the settled outcome (applied, or
  /// rejected with nothing written) under its own request id.
  Future<String> relay(String message, String frameUrl) async {
    final raw = _map(
      await call('POST', '$_base/${Uri.encodeComponent(_id)}/messages', {
        'frameUrl': frameUrl,
        'message': message,
      }),
    );
    var answer = raw ?? const <String, dynamic>{};
    final proposal = PluginActionOutcome.fromJson(answer).proposal;
    if (answer['ok'] == true && proposal != null) {
      final apply = await confirm(proposal);
      final settled = _map(
        await call(
          'POST',
          '/api/plugin-actions/proposals/${Uri.encodeComponent(proposal.id)}/${apply ? 'apply' : 'reject'}',
        ),
      );
      answer = {...?settled, 'requestId': answer['requestId']};
    }
    return jsonEncode(answer);
  }

  /// What to do with a URL the page asked to open.
  Future<PluginWebNavigation> navigate(String url) async {
    final raw = _map(
      await call('POST', '$_base/${Uri.encodeComponent(_id)}/navigate', {
        'url': url,
      }),
    );
    return switch (raw?['decision']) {
      'same-origin' => PluginWebNavigation.sameOrigin,
      'external' => PluginWebNavigation.external,
      _ => PluginWebNavigation.deny,
    };
  }

  /// Ends the session; its later messages are refused.
  Future<void> close() async {
    if (_id.isEmpty) return;
    final id = _id;
    _id = '';
    await call('DELETE', '$_base/${Uri.encodeComponent(id)}');
  }
}

/// Draws the page the session opened. The default is the system WebView;
/// tests inject their own.
typedef PluginWebRendererBuilder = Widget Function(
  BuildContext context,
  PluginWebSession session,
  PluginWebOpening opening,
);

/// A plugin page whose renderer is `web-ui`: it opens a session for this
/// platform and draws the page, or the native fallback with the daemon's reason
/// when the platform, the plugin or its resources are not ready.
class PluginWebPage extends StatefulWidget {
  const PluginWebPage({
    super.key,
    required this.page,
    this.call,
    this.platform,
    this.renderer,
  });

  final PluginPageContribution page;

  /// The daemon call; null uses the app's API.
  final ApiCall? call;

  /// The platform sent to the daemon; null uses [pluginWebPlatform].
  final String? platform;
  final PluginWebRendererBuilder? renderer;

  @override
  State<PluginWebPage> createState() => _PluginWebPageState();
}

class _PluginWebPageState extends State<PluginWebPage> {
  late final PluginWebSession _session;
  PluginWebOpening? _opening;
  Object? _error;

  @override
  void initState() {
    super.initState();
    _session = PluginWebSession(
      call: widget.call ?? context.read<AppState>().api.call,
      confirm: _confirm,
    );
    _open();
  }

  Future<void> _open() async {
    try {
      final opening = await _session.open(
        pluginId: widget.page.pluginId,
        pageId: widget.page.id,
        platform: widget.platform ?? pluginWebPlatform(),
      );
      if (mounted) setState(() => _opening = opening);
    } catch (e) {
      if (mounted) setState(() => _error = e);
    }
  }

  @override
  void dispose() {
    unawaited(_session.close().catchError((_) {}));
    super.dispose();
  }

  Future<bool> _confirm(PluginProposal proposal) async {
    if (!mounted) return false;
    final approved = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.pluginProposalTitle(proposal.summary)),
        content: Text(l10n.pluginProposalBody(widget.page.title)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: Text(l10n.pluginProposalReject),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: Text(l10n.pluginProposalApply),
          ),
        ],
      ),
    );
    return approved == true;
  }

  /// The native fallback; a page that declares a `document` offers it as its
  /// native page.
  Widget _fallback(
    PluginWebAdmission admission,
    PluginWebFallbackReason reason,
    String detail,
  ) {
    final document = widget.page.view['document'];
    return PluginWebFallbackPage(
      pluginTitle: widget.page.title,
      admission: admission,
      reason: reason,
      detail: detail.isEmpty ? null : detail,
      onOpenNative: document is Map
          ? () => Navigator.of(context).push(
              MaterialPageRoute<void>(
                builder: (_) => Scaffold(
                  appBar: AppBar(title: Text(widget.page.title)),
                  body: PluginDeclarativeView(
                    pluginId: widget.page.pluginId,
                    viewId: widget.page.id,
                    document: document.cast<String, dynamic>(),
                    call: widget.call,
                  ),
                ),
              ),
            )
          : null,
    );
  }

  @override
  Widget build(BuildContext context) {
    final opening = _opening;
    if (_error != null) {
      return _fallback(
        PluginWebAdmission.needsReview,
        PluginWebFallbackReason.resourcesMissing,
        l10n.pluginWebOpenFailed('$_error'),
      );
    }
    if (opening == null) {
      return Scaffold(
        appBar: AppBar(title: Text(widget.page.title)),
        body: Center(
          key: const ValueKey('plugin-web-loading'),
          child: Text(l10n.pluginWebLoading),
        ),
      );
    }
    if (!opening.renders) {
      return _fallback(
        opening.admission,
        opening.fallback ?? PluginWebFallbackReason.resourcesMissing,
        opening.detail,
      );
    }
    final renderer = widget.renderer ?? _systemWebView;
    return Scaffold(
      appBar: AppBar(title: Text(widget.page.title)),
      body: Column(
        children: [
          if (opening.preview) const _PreviewNotice(),
          Expanded(child: renderer(context, _session, opening)),
        ],
      ),
    );
  }
}

Widget _systemWebView(
  BuildContext context,
  PluginWebSession session,
  PluginWebOpening opening,
) => PluginSystemWebView(session: session, opening: opening);

class _PreviewNotice extends StatelessWidget {
  const _PreviewNotice();

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      key: const ValueKey('plugin-web-preview'),
      width: double.infinity,
      color: cs.tertiaryContainer,
      padding: const EdgeInsets.symmetric(
        horizontal: Space.lg,
        vertical: Space.sm,
      ),
      child: Text(
        l10n.pluginWebPreview,
        style: Theme.of(context).textTheme.bodySmall
            ?.copyWith(color: cs.onTertiaryContainer),
      ),
    );
  }
}

/// The system WebView renderer (Android System WebView, macOS WKWebView).
///
/// JavaScript runs only in the page the daemon built; device permissions are
/// denied, file and content access are off, mixed content is never allowed, and
/// every navigation after the first load is answered by the daemon and
/// prevented — an external https link opens in the system browser after the
/// user confirms. The frame reported with each message is the base URL this
/// renderer loaded, or any other URL the WebView reported since, so a message
/// from a page that is not the session's is refused by the daemon. Android
/// reports a page loaded from a string as `about:blank`, which therefore counts
/// as the loaded page.
class PluginSystemWebView extends StatefulWidget {
  const PluginSystemWebView({
    super.key,
    required this.session,
    required this.opening,
  });

  final PluginWebSession session;
  final PluginWebOpening opening;

  @override
  State<PluginSystemWebView> createState() => _PluginSystemWebViewState();
}

class _PluginSystemWebViewState extends State<PluginSystemWebView> {
  late final WebViewController _controller;
  late String _frame = widget.opening.baseUrl;
  bool _loaded = false;

  @override
  void initState() {
    super.initState();
    final base = widget.opening.baseUrl;
    _controller = WebViewController(
      onPermissionRequest: (request) => request.deny(),
    );
    final platform = _controller.platform;
    if (platform is AndroidWebViewController) {
      unawaited(platform.setAllowFileAccess(false));
      unawaited(platform.setAllowContentAccess(false));
      unawaited(platform.setGeolocationEnabled(false));
      unawaited(platform.setMixedContentMode(MixedContentMode.neverAllow));
    }
    unawaited(_controller.setJavaScriptMode(JavaScriptMode.unrestricted));
    unawaited(
      _controller.addJavaScriptChannel(
        'CoursiveBridge',
        onMessageReceived: (message) => _relay(message.message),
      ),
    );
    unawaited(
      _controller.setNavigationDelegate(
        NavigationDelegate(
          onNavigationRequest: _navigate,
          onUrlChange: (change) {
            final url = change.url;
            if (url != null && url != base && url != 'about:blank') {
              _frame = url;
            }
          },
          onPageFinished: (_) => _loaded = true,
        ),
      ),
    );
    unawaited(_controller.loadHtmlString(widget.opening.html, baseUrl: base));
  }

  Future<void> _relay(String message) async {
    String answer;
    try {
      answer = await widget.session.relay(message, _frame);
    } catch (e) {
      answer = jsonEncode({
        'ok': false,
        'error': {'code': 'unavailable', 'message': '$e'},
      });
    }
    // jsonEncode of the JSON text is a JavaScript string literal.
    await _controller.runJavaScript(
      'window.__coursiveReceive(${jsonEncode(answer)})',
    );
  }

  Future<NavigationDecision> _navigate(NavigationRequest request) async {
    final base = Uri.parse(widget.opening.baseUrl);
    final target = Uri.tryParse(request.url);
    if (!_loaded && (request.url == 'about:blank' || target == base)) {
      return NavigationDecision.navigate;
    }
    final decision = await widget.session
        .navigate(request.url)
        .catchError((_) => PluginWebNavigation.deny);
    switch (decision) {
      case PluginWebNavigation.sameOrigin:
        // The page is one document: a fragment change stays, a load does not.
        if (target != null && target.removeFragment() == base) {
          return NavigationDecision.navigate;
        }
      case PluginWebNavigation.external:
        if (target != null) unawaited(_openExternal(target));
        return NavigationDecision.prevent;
      case PluginWebNavigation.deny:
        break;
    }
    if (mounted) {
      ScaffoldMessenger.maybeOf(context)?.showSnackBar(
        SnackBar(content: Text(l10n.pluginWebNavigationBlocked(request.url))),
      );
    }
    return NavigationDecision.prevent;
  }

  Future<void> _openExternal(Uri target) async {
    final open = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.pluginWebExternalTitle(target.host)),
        content: Text(l10n.pluginWebExternalBody(target.toString())),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: Text(l10n.commonCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: Text(l10n.pluginWebExternalOpen),
          ),
        ],
      ),
    );
    if (open == true) {
      await launchUrl(target, mode: LaunchMode.externalApplication);
    }
  }

  @override
  Widget build(BuildContext context) => WebViewWidget(controller: _controller);
}

/// The page builder for a `web-ui` contribution: the system WebView where the
/// daemon admits this platform, the native fallback everywhere else.
Widget pluginWebPageBuilder(
  BuildContext context,
  PluginPageContribution page,
) => PluginWebPage(page: page);

Map<String, dynamic>? _map(Object? v) =>
    v is Map ? v.cast<String, dynamic>() : null;
