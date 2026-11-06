import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';

import '../../core/api.dart';
import '../../core/app_state.dart';
import '../../core/models.dart';
import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';
import '../../widgets/adaptive_sheet.dart';

/// Signs a source in with the form its provider declares, then any
/// challenge it asks for. Returns true once signed in.
/// Contract: docs/specs/0024-source-provider-plugins.md
Future<bool> showSignInSheet(
  BuildContext context,
  Source source, {
  ApiCall? api,
}) async =>
    await showAdaptiveSheet<bool>(
      context,
      dismissible: false,
      builder: (_) => SignInSheet(source: source, api: api),
    ) ??
    false;

class SignInSheet extends StatefulWidget {
  const SignInSheet({super.key, required this.source, this.api});
  final Source source;

  /// For tests; defaults to the app's daemon connection.
  final ApiCall? api;

  @override
  State<SignInSheet> createState() => _SignInSheetState();
}

class _SignInSheetState extends State<SignInSheet> {
  late final _values = {
    for (final f in widget.source.fields) f.key: TextEditingController(),
  };
  final _answer = TextEditingController();
  final _form = GlobalKey<FormState>();
  late SignInChallenge? _challenge = widget.source.challenge;
  bool _busy = false;
  String? _error;

  ApiCall get _call => widget.api ?? context.read<AppState>().api.call;
  String get _path => '/api/sources/${widget.source.id}';

  @override
  void dispose() {
    for (final c in _values.values) {
      c.dispose();
    }
    _answer.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_busy || !_form.currentState!.validate()) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final ch = _challenge;
      final r = ch == null
          ? await _call('POST', '$_path/sign-in', {
              'fields': {
                for (final e in _values.entries) e.key: e.value.text.trim(),
              },
            })
          : await _call('POST', '$_path/answer', {
              'challenge': ch.id,
              'answer': _answer.text.trim(),
            });
      if (!mounted) return;
      final auth = (r as Map).cast<String, dynamic>();
      final next = switch (auth['challenge']) {
        final Map c => SignInChallenge(c.cast<String, dynamic>()),
        _ => null,
      };
      if (auth['state'] == 'signed_in') {
        Navigator.pop(context, true);
        return;
      }
      _answer.clear();
      setState(() {
        _challenge = next;
        if (next == null) _error = l10n.sourceSignInIncomplete;
      });
    } catch (e) {
      if (mounted) setState(() => _error = '$e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final tt = Theme.of(context).textTheme;
    final cs = Theme.of(context).colorScheme;
    final ch = _challenge;
    final fields = widget.source.fields;
    return SingleChildScrollView(
      padding: sheetPadding(context),
      child: Form(
        key: _form,
        child: AutofillGroup(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                l10n.sourceSignInTitle(widget.source.title),
                style: tt.headlineSmall,
              ),
              const SizedBox(height: Space.sm),
              Text(
                ch == null ? l10n.sourceSignInBody : ch.prompt,
                style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant),
              ),
              const SizedBox(height: Space.xl),
              if (ch == null)
                for (final (i, f) in fields.indexed) ...[
                  if (i > 0) const SizedBox(height: Space.lg),
                  _field(f, last: i == fields.length - 1),
                ]
              else
                ..._challengeStep(ch),
              if (_error != null) ...[
                const SizedBox(height: Space.lg),
                InlineError(text: _error!),
              ],
              const SizedBox(height: Space.xl),
              Row(
                children: [
                  const Spacer(),
                  TextButton(
                    onPressed: _busy
                        ? null
                        : () => Navigator.pop(context, false),
                    child: Text(l10n.commonCancel),
                  ),
                  const SizedBox(width: Space.sm),
                  FilledButton(
                    onPressed: _busy ? null : _submit,
                    child: _busy
                        ? SizedBox.square(
                            dimension: 20,
                            child: CircularProgressIndicator(
                              strokeWidth: 2.5,
                              color: cs.onPrimary,
                            ),
                          )
                        : Text(
                            ch == null
                                ? l10n.loginSignIn
                                : l10n.sourceSignInContinue,
                          ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _field(SignInField f, {required bool last}) => TextFormField(
    controller: _values[f.key],
    autofocus: f == widget.source.fields.first,
    enabled: !_busy,
    obscureText: f.kind == 'password',
    keyboardType: f.kind == 'otp' ? TextInputType.number : null,
    autofillHints: switch (f.kind) {
      'password' => const [AutofillHints.password],
      'otp' => const [AutofillHints.oneTimeCode],
      _ => const [AutofillHints.username],
    },
    textInputAction: last ? TextInputAction.done : TextInputAction.next,
    onFieldSubmitted: last ? (_) => _submit() : null,
    decoration: InputDecoration(
      labelText: f.title,
      border: const OutlineInputBorder(),
      helperText: f.remember ? null : l10n.sourceFieldNotStored,
    ),
    validator: (v) => v == null || v.trim().isEmpty
        ? l10n.sourceFieldRequired(f.title)
        : null,
  );

  List<Widget> _challengeStep(SignInChallenge ch) {
    final bytes = _dataUrl(ch.image);
    return [
      if (bytes != null)
        Center(
          child: ClipRRect(
            borderRadius: BorderRadius.circular(Corners.sm),
            // A challenge image is shown at its natural size.
            child: Image.memory(
              bytes,
              filterQuality: FilterQuality.none,
              semanticLabel: ch.prompt,
            ),
          ),
        ),
      if (bytes != null && ch.kind != 'qr') const SizedBox(height: Space.lg),
      if (ch.kind != 'qr')
        TextFormField(
          controller: _answer,
          autofocus: true,
          enabled: !_busy,
          textInputAction: TextInputAction.done,
          onFieldSubmitted: (_) => _submit(),
          inputFormatters: [LengthLimitingTextInputFormatter(64)],
          decoration: InputDecoration(
            labelText: l10n.sourceChallengeAnswer,
            border: const OutlineInputBorder(),
          ),
          validator: (v) => v == null || v.trim().isEmpty
              ? l10n.sourceFieldRequired(l10n.sourceChallengeAnswer)
              : null,
        ),
    ];
  }
}

Uint8List? _dataUrl(String url) {
  final i = url.indexOf(';base64,');
  if (!url.startsWith('data:image/') || i < 0) return null;
  try {
    return base64Decode(url.substring(i + 8));
  } on FormatException {
    return null;
  }
}
