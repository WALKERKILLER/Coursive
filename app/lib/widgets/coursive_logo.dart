import 'package:flutter/material.dart';

/// The app's logo, the launcher icon itself: a rounded tile exported from
/// the brand master (assets/brand in the repository root).
class CoursiveLogo extends StatelessWidget {
  const CoursiveLogo({super.key, required this.size, this.label = 'Coursive'});
  final double size;

  /// Read by screen readers; null when the name is written beside the mark.
  final String? label;

  static const asset = 'branding/logo.png';

  @override
  Widget build(BuildContext context) => Image.asset(
        asset,
        width: size,
        height: size,
        filterQuality: FilterQuality.medium,
        semanticLabel: label,
        excludeFromSemantics: label == null,
      );
}
