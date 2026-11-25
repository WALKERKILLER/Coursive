import 'package:flutter/material.dart';

/// Read surfaces keep their standard controls across installed app combinations.
Iterable<ContextMenuButtonItem> readingSelectionActions(
  SelectableRegionState region,
) => region.contextMenuButtonItems.where(
  (item) => switch (item.type) {
    ContextMenuButtonType.copy ||
    ContextMenuButtonType.share ||
    ContextMenuButtonType.selectAll => true,
    _ => false,
  },
);

Widget readingSelectionMenu(
  BuildContext context,
  SelectableRegionState region,
) {
  return AdaptiveTextSelectionToolbar.buttonItems(
    anchors: region.contextMenuAnchors,
    buttonItems: readingSelectionActions(region).toList(),
  );
}
