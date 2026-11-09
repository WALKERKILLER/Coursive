import 'dart:async';
import 'dart:math';

import '../../core/api.dart';

/// The typed action bridge of the UI Extension Protocol: a declarative view
/// sends `{pluginId, actionId, viewId, instanceId, args, contextRevision,
/// requestId}` with a deadline and a trace id, and the daemon answers with one
/// of statePatch, navigation, viewPatch, notification or a pending proposal
/// (docs/specs/0032-ui-extension-protocol.md).
///
/// This client only moves the typed envelope and hands each result form to an
/// injected callback; how a view is drawn stays with the renderer.
class PluginActionRequest {
  PluginActionRequest({
    required this.pluginId,
    required this.actionId,
    this.viewId = '',
    this.instanceId = '',
    this.args = const {},
    this.contextRevision = 0,
    this.deadlineMs = defaultDeadlineMs,
    String? requestId,
    this.traceId = '',
  }) : requestId = requestId ?? newPluginActionRequestId();

  /// The daemon's default deadline, for a caller that names none.
  static const defaultDeadlineMs = 10000;

  final String pluginId;
  final String actionId;

  /// The declarative view the action came from; its revision is the one
  /// [contextRevision] carries.
  final String viewId;

  /// One mounted instance of the view, so two pages of one plugin stay apart.
  final String instanceId;
  final Map<String, dynamic> args;

  /// The revision the caller read; the daemon refuses a stale one.
  final int contextRevision;

  /// Retries and repeated clicks reuse this id, so the daemon answers once.
  final String requestId;
  final int deadlineMs;
  final String traceId;

  Map<String, dynamic> toJson() => {
    'pluginId': pluginId,
    'actionId': actionId,
    'viewId': viewId,
    'instanceId': instanceId,
    'args': args,
    'contextRevision': contextRevision,
    'requestId': requestId,
    'deadlineMs': deadlineMs,
    'traceId': traceId,
  };
}

/// A fresh request id; the daemon replays an answer for a repeated one.
String newPluginActionRequestId() {
  final tail = Random().nextInt(1 << 32).toRadixString(36);
  return 'req-${DateTime.now().microsecondsSinceEpoch.toRadixString(36)}-$tail';
}

/// The daemon's answer: a result form or a typed error, never both.
class PluginActionOutcome {
  PluginActionOutcome({
    required this.requestId,
    required this.traceId,
    this.statePatch,
    this.navigation,
    this.viewPatch,
    this.notification,
    this.proposal,
    this.failure,
  });

  factory PluginActionOutcome.fromJson(Map<String, dynamic> json) {
    final result = _map(json['result']);
    return PluginActionOutcome(
      requestId: '${json['requestId'] ?? ''}',
      traceId: '${json['traceId'] ?? ''}',
      statePatch: result == null ? null : _statePatch(result['statePatch']),
      navigation: result == null ? null : _navigation(result['navigation']),
      viewPatch: result == null ? null : _viewPatch(result['viewPatch']),
      notification: result == null
          ? null
          : _notification(result['notification']),
      proposal: result == null ? null : _proposal(result['proposal']),
      failure: _failure(json['error']),
    );
  }

  final String requestId;
  final String traceId;
  final PluginStatePatch? statePatch;
  final PluginNavigation? navigation;
  final PluginViewPatch? viewPatch;
  final PluginNotification? notification;
  final PluginProposal? proposal;

  /// The typed refusal; [ok] is false exactly when it is set.
  final PluginActionFailure? failure;

  bool get ok => failure == null;

  static PluginStatePatch? _statePatch(Object? v) {
    final m = _map(v);
    if (m == null) return null;
    return PluginStatePatch(
      viewId: '${m['viewId'] ?? ''}',
      revision: _int(m['revision']),
      idempotencyKey: '${m['idempotencyKey'] ?? ''}',
      patch: _map(m['patch']) ?? const {},
      state: _map(m['state']) ?? const {},
    );
  }

  static PluginNavigation? _navigation(Object? v) {
    final m = _map(v);
    if (m == null) return null;
    return PluginNavigation(
      route: '${m['route'] ?? ''}',
      title: '${m['title'] ?? ''}',
      replace: m['replace'] == true,
    );
  }

  static PluginViewPatch? _viewPatch(Object? v) {
    final m = _map(v);
    if (m == null) return null;
    return PluginViewPatch(
      viewId: '${m['viewId'] ?? ''}',
      revision: _int(m['revision']),
      patch: _map(m['patch']) ?? const {},
    );
  }

  static PluginNotification? _notification(Object? v) {
    final m = _map(v);
    if (m == null) return null;
    return PluginNotification(
      text: '${m['text'] ?? ''}',
      level: '${m['level'] ?? 'info'}',
    );
  }

  static PluginProposal? _proposal(Object? v) {
    final m = _map(v);
    if (m == null) return null;
    return PluginProposal(
      id: '${m['id'] ?? ''}',
      pluginId: '${m['pluginId'] ?? ''}',
      actionId: '${m['actionId'] ?? ''}',
      viewId: '${m['viewId'] ?? ''}',
      summary: '${m['summary'] ?? ''}',
      approvalMode: '${m['approvalMode'] ?? ''}',
      status: '${m['status'] ?? ''}',
      preview: _map(m['preview']) ?? const {},
    );
  }

  static PluginActionFailure? _failure(Object? v) {
    final m = _map(v);
    if (m == null) return null;
    return PluginActionFailure(
      code: '${m['code'] ?? ''}',
      message: '${m['message'] ?? ''}',
      details: _map(m['details']) ?? const {},
      recoverable: m['recoverable'] == true,
    );
  }
}

/// A refused action. [recoverable] marks the cases a view can retry after
/// reloading its state: a stale revision, a timeout or a cancellation.
class PluginActionFailure {
  PluginActionFailure({
    required this.code,
    required this.message,
    this.details = const {},
    this.recoverable = false,
  });
  final String code;
  final String message;
  final Map<String, dynamic> details;
  final bool recoverable;

  @override
  String toString() => message.isEmpty ? code : message;
}

/// A view state patch the daemon applied; [state] is the merged result at
/// [revision], and [idempotencyKey] names the write that produced it.
class PluginStatePatch {
  PluginStatePatch({
    required this.viewId,
    required this.revision,
    required this.idempotencyKey,
    required this.patch,
    required this.state,
  });
  final String viewId;
  final int revision;
  final String idempotencyKey;
  final Map<String, dynamic> patch;
  final Map<String, dynamic> state;
}

/// A view's plugin state at its current revision.
class PluginViewState {
  PluginViewState({
    required this.viewId,
    required this.revision,
    required this.state,
  });
  final String viewId;
  final int revision;
  final Map<String, dynamic> state;
}

/// A request to open a route the host already knows.
class PluginNavigation {
  PluginNavigation({
    required this.route,
    this.title = '',
    this.replace = false,
  });
  final String route;
  final String title;
  final bool replace;
}

/// A partial replacement of a rendered view.
class PluginViewPatch {
  PluginViewPatch({
    required this.viewId,
    required this.revision,
    required this.patch,
  });
  final String viewId;
  final int revision;
  final Map<String, dynamic> patch;
}

/// A short message for the user.
class PluginNotification {
  PluginNotification({required this.text, this.level = 'info'});
  final String text;

  /// info, warning or error.
  final String level;
}

/// A write the user has not authorized yet. It is shown, never applied: the
/// existing approval path commits it.
class PluginProposal {
  PluginProposal({
    required this.id,
    required this.pluginId,
    required this.actionId,
    this.viewId = '',
    this.summary = '',
    this.approvalMode = '',
    this.status = '',
    this.preview = const {},
  });
  final String id;
  final String pluginId;
  final String actionId;
  final String viewId;
  final String summary;
  final String approvalMode;
  final String status;
  final Map<String, dynamic> preview;
}

/// Sends typed actions through the daemon and applies each result form to the
/// callbacks the caller injected.
///
/// [call] is `Api.call`, so the client shares the daemon token and base URL
/// (`PluginActionClient(call: api.call, …)`). A request id is sent at most
/// once: a repeated click or a retry gets the first answer, and a settled
/// answer is replayed without a second dispatch.
class PluginActionClient {
  PluginActionClient({
    required this.call,
    this.onStatePatch,
    this.onNavigation,
    this.onViewPatch,
    this.onNotification,
    this.onProposal,
    this.keep = 64,
  });

  final ApiCall call;
  final void Function(PluginStatePatch patch)? onStatePatch;
  final void Function(PluginNavigation navigation)? onNavigation;
  final void Function(PluginViewPatch patch)? onViewPatch;
  final void Function(PluginNotification notification)? onNotification;
  final void Function(PluginProposal proposal)? onProposal;

  /// How many settled answers are remembered for replay.
  final int keep;

  final _inFlight = <String, Future<PluginActionOutcome>>{};
  final _settled = <String, PluginActionOutcome>{};
  final _revisions = <String, int>{};

  /// The last revision the daemon reported for a plugin view; send it as the
  /// next request's `contextRevision`.
  int revisionOf(String pluginId, String viewId) =>
      _revisions['$pluginId/$viewId'] ?? 0;

  /// Reads a view's state and revision from the daemon and remembers the
  /// revision, so a view that mounts again is not refused as stale.
  Future<PluginViewState> readState(String pluginId, String viewId) async {
    final raw = await call(
      'GET',
      '/api/plugin-actions/state?pluginId=${Uri.encodeQueryComponent(pluginId)}'
          '&viewId=${Uri.encodeQueryComponent(viewId)}',
    );
    final m = _map(raw) ?? const {};
    final view = PluginViewState(
      viewId: '${m['viewId'] ?? viewId}',
      revision: _int(m['revision']),
      state: _map(m['state']) ?? const {},
    );
    _revisions['$pluginId/${view.viewId}'] = view.revision;
    return view;
  }

  /// Sends [request] once. The answer is dispatched to the callbacks, and a
  /// transport failure (no answer at all) is thrown, not remembered, so a
  /// retry can still reach the daemon.
  Future<PluginActionOutcome> invoke(PluginActionRequest request) {
    final settled = _settled[request.requestId];
    if (settled != null) return Future.value(settled);
    final running = _inFlight[request.requestId];
    if (running != null) return running;
    late final Future<PluginActionOutcome> sent;
    // A block body: returning the stored future from whenComplete would make
    // the completion wait on itself.
    sent = _send(request).whenComplete(() {
      _inFlight.remove(request.requestId);
    });
    _inFlight[request.requestId] = sent;
    return sent;
  }

  Future<PluginActionOutcome> _send(PluginActionRequest request) async {
    final raw = await call('POST', '/api/plugin-actions', request.toJson());
    final outcome = PluginActionOutcome.fromJson(
      raw is Map ? raw.cast<String, dynamic>() : const {},
    );
    // A timeout or a cancellation wrote nothing, so it is not settled: a retry
    // with the same requestId may still reach the daemon.
    final failure = outcome.failure;
    final retryable =
        failure != null &&
        (failure.code == 'deadline_exceeded' || failure.code == 'canceled');
    if (!retryable) {
      _settled[request.requestId] = outcome;
      if (_settled.length > keep) _settled.remove(_settled.keys.first);
    }
    if (outcome.ok) _dispatch(request, outcome);
    return outcome;
  }

  /// Applies the one result form the daemon allows, keeping the view revision
  /// current so the next action is not refused as stale.
  void _dispatch(PluginActionRequest request, PluginActionOutcome outcome) {
    final patch = outcome.statePatch;
    if (patch != null) {
      _revisions['${request.pluginId}/${patch.viewId}'] = patch.revision;
      onStatePatch?.call(patch);
    }
    final viewPatch = outcome.viewPatch;
    if (viewPatch != null) {
      _revisions['${request.pluginId}/${viewPatch.viewId}'] =
          viewPatch.revision;
      onViewPatch?.call(viewPatch);
    }
    if (outcome.navigation case final navigation?) {
      onNavigation?.call(navigation);
    }
    if (outcome.notification case final notification?) {
      onNotification?.call(notification);
    }
    if (outcome.proposal case final proposal?) {
      onProposal?.call(proposal);
    }
  }
}

Map<String, dynamic>? _map(Object? v) =>
    v is Map ? v.cast<String, dynamic>() : null;

int _int(Object? v) => v is num ? v.toInt() : 0;
