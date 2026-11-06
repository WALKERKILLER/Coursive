import 'package:flutter/material.dart';

import '../theme/tokens.dart';

/// Shows an editor the way the schedule editor does: a dialog on wide
/// windows, a scrollable bottom sheet on compact ones. A sheet that is not
/// [draggable] still closes from its barrier, which honours `PopScope`;
/// dragging it closed would not.
Future<T?> showAdaptiveSheet<T>(
  BuildContext context, {
  required WidgetBuilder builder,
  double maxWidth = 520,
  bool dismissible = true,
  bool? draggable,
}) {
  if (!context.isCompact) {
    return showDialog<T>(
      context: context,
      useRootNavigator: true,
      barrierDismissible: dismissible,
      builder: (c) => Dialog(
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(Corners.xl),
        ),
        clipBehavior: Clip.antiAlias,
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: maxWidth),
          child: builder(c),
        ),
      ),
    );
  }
  return showModalBottomSheet<T>(
    context: context,
    useRootNavigator: true,
    isScrollControlled: true,
    isDismissible: dismissible,
    enableDrag: draggable ?? dismissible,
    showDragHandle: draggable ?? dismissible,
    builder: builder,
  );
}

/// The padding of a sheet's scrollable body, clear of the keyboard. A sheet
/// without a drag [handle] keeps the full top padding.
EdgeInsets sheetPadding(BuildContext context, {bool handle = true}) =>
    EdgeInsets.fromLTRB(
      Space.xl,
      context.isCompact && handle ? 0 : Space.xl,
      Space.xl,
      Space.xl + MediaQuery.viewInsetsOf(context).bottom,
    );

/// An inline failure message, next to the action that failed.
class InlineError extends StatelessWidget {
  const InlineError({super.key, required this.text});
  final String text;
  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: Space.lg,
        vertical: Space.md,
      ),
      decoration: BoxDecoration(
        color: cs.errorContainer,
        borderRadius: BorderRadius.circular(Corners.md),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            Icons.error_outline_rounded,
            size: 20,
            color: cs.onErrorContainer,
          ),
          const SizedBox(width: Space.md),
          Expanded(
            child: Text(
              text,
              style: Theme.of(context).textTheme.bodyMedium
                  ?.copyWith(color: cs.onErrorContainer),
            ),
          ),
        ],
      ),
    );
  }
}
