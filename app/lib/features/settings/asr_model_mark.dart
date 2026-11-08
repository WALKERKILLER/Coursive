import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';

/// A brand icon: the asset and whether it is single-colour, tinted with
/// onSurface.
typedef AsrMark = ({String asset, bool mono});

/// The maker of each local model family, matched by catalog id prefix so new
/// sizes of a family keep their icon (see NOTICE in assets/providers and
/// assets/brands).
const _families = <(String, AsrMark)>[
  ('qwen', (asset: 'assets/providers/qwen-color.svg', mono: false)),
  ('sensevoice', (asset: 'assets/providers/alibaba-color.svg', mono: false)),
  ('paraformer', (asset: 'assets/providers/alibaba-color.svg', mono: false)),
  ('fun-asr', (asset: 'assets/providers/alibaba-color.svg', mono: false)),
  ('firered', (asset: 'assets/brands/firered.png', mono: false)),
  ('whisper', (asset: 'assets/providers/openai.svg', mono: true)),
];

AsrMark? asrModelMark(Object? model) {
  final id = '${model ?? ''}';
  for (final (prefix, mark) in _families) {
    if (id.startsWith(prefix)) {
      return mark;
    }
  }
  return null;
}

/// The icon alone, [size] square; bitmap icons get rounded corners.
class AsrMarkImage extends StatelessWidget {
  const AsrMarkImage(this.mark, {super.key, this.size = 20, this.label});
  final AsrMark mark;
  final double size;
  final String? label;

  @override
  Widget build(BuildContext context) {
    if (mark.asset.endsWith('.svg')) {
      return SvgPicture.asset(
        mark.asset,
        width: size,
        height: size,
        semanticsLabel: label,
        colorFilter: mark.mono ? ColorFilter.mode(Theme.of(context).colorScheme.onSurface, BlendMode.srcIn) : null,
      );
    }
    return ClipRRect(
      borderRadius: BorderRadius.circular(size * 0.25),
      child: Image.asset(mark.asset, width: size, height: size, fit: BoxFit.cover, filterQuality: FilterQuality.medium, semanticLabel: label),
    );
  }
}
