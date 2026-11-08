// Formulas in an outline: the review and the PDF read the same runs, and
// the review sets them as math rather than TeX source.
import 'package:flutter_test/flutter_test.dart';
import 'package:val_latex_flutter/val_latex_flutter.dart';

import '../lib/features/study_canvas/outline_document.dart';
import 'study_grounded_answers_test.dart' as grounded;
import 'study_outline_review_test.dart' as review;

String runs(String text) =>
    [for (final r in outlineInline(text)) '${r.kind.name}:${r.text}']
        .join(' | ');

void main() {
  test('every formula form is found and prices stay text', () {
    expect(
      runs(r'**粗** `c` $a^2$ \(b\) $$c$$ \[d\] 价格 $5 和 $6'),
      'bold:粗 | text:  | code:c | text:  | math:a^2 | text:  | math:b | '
      r'text:  | displayMath:c | text:  | displayMath:d | text: 价格 $5 和 $6',
    );
    // A `$$` inside a sentence is one display formula, not two stray `$`.
    expect(runs(r'即 $$x=1$$。'), 'text:即  | displayMath:x=1 | text:。');
    expect(runs(r'$ x $ 不是公式'), r'text:$ x $ 不是公式');
  });

  testWidgets('the review sets an item\'s formulas as math', (t) async {
    final turn = review.outlineTurn();
    final items =
        ((((turn['result'] as Map)['outline_proposal'] as Map)['sections']
                        as List)
                    .first
                as Map)['items']
            as List;
    (items[2] as Map)['text'] = r'为什么 $t_{miss}=t_{hit}+p$ 成立？';
    await review.openReview(t, turn: turn);
    final math = find.descendant(
      of: review.row('i3'),
      matching: find.byType(Math),
    );
    expect(math, findsOneWidget);
    expect(
      find.descendant(
        of: review.row('i3'),
        matching: find.textContaining(r'$t_'),
      ),
      findsNothing,
    );
    expect(
      find.descendant(
        of: review.row('i3'),
        matching: find.textContaining('成立'),
      ),
      findsOneWidget,
    );
    await grounded.close(t);
  });
}
