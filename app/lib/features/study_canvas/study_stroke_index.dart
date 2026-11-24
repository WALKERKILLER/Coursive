part of 'study_document.dart';

/// Strokes keep their document order. Nearby handwriting is usually consecutive,
/// so a block outside the query can be skipped without visiting its samples.
class _StrokeBlock {
  _StrokeBlock(List<StudyStroke> strokes, int start, int end) {
    for (var i = start; i < end; i++) {
      final stroke = strokes[i];
      points += stroke.points.length;
      if (stroke.points.isEmpty) continue;
      bounds = bounds?.expandToInclude(stroke.bounds) ?? stroke.bounds;
      if (stroke.anchor.isNotEmpty) anchors.add(stroke.anchor);
    }
  }
  Rect? bounds;
  int points = 0;
  final anchors = <String>{};
}

class _IndexedStrokes extends ListBase<StudyStroke> {
  _IndexedStrokes(Iterable<StudyStroke> values) : _items = List.of(values);
  static const _blockSize = 64;
  final List<StudyStroke> _items;
  final _blocks = <_StrokeBlock>[];
  int _indexedPoints = 0;

  void _invalidateFrom(int position) {
    final first = position ~/ _blockSize;
    while (_blocks.length > first) {
      _indexedPoints -= _blocks.removeLast().points;
    }
  }

  void _index() {
    for (var i = _blocks.length * _blockSize; i < length; i += _blockSize) {
      final block = _StrokeBlock(_items, i, math.min(i + _blockSize, length));
      _blocks.add(block);
      _indexedPoints += block.points;
    }
  }

  List<StudyStroke> snapshot() => List.of(_items);

  int get pointCount {
    _index();
    return _indexedPoints;
  }

  Iterable<int> intersecting(Rect area) sync* {
    _index();
    for (var block = 0; block < _blocks.length; block++) {
      if (_blocks[block].bounds?.overlaps(area) != true) continue;
      final end = math.min((block + 1) * _blockSize, length);
      for (var i = block * _blockSize; i < end; i++) {
        if (_items[i].points.isNotEmpty && _items[i].bounds.overlaps(area))
          yield i;
      }
    }
  }

  Iterable<StudyStroke> anchored(String id) sync* {
    _index();
    for (var block = 0; block < _blocks.length; block++) {
      if (!_blocks[block].anchors.contains(id) && id.isNotEmpty) continue;
      final end = math.min((block + 1) * _blockSize, length);
      for (var i = block * _blockSize; i < end; i++) {
        if (_items[i].anchor == id) yield _items[i];
      }
    }
  }

  @override
  int get length => _items.length;
  @override
  set length(int value) {
    final old = length;
    _items.length = value;
    _invalidateFrom(math.min(value, old));
  }

  @override
  StudyStroke operator [](int index) => _items[index];
  @override
  void operator []=(int index, StudyStroke value) {
    _items[index] = value;
    _invalidateFrom(index);
  }

  @override
  void add(StudyStroke value) {
    _invalidateFrom(length);
    _items.add(value);
  }

  @override
  void addAll(Iterable<StudyStroke> values) {
    _invalidateFrom(length);
    _items.addAll(values);
  }

  @override
  void insert(int index, StudyStroke value) =>
      replaceRange(index, index, [value]);
  @override
  void insertAll(int index, Iterable<StudyStroke> values) =>
      replaceRange(index, index, values);
  @override
  void removeRange(int start, int end) => replaceRange(start, end, const []);
  @override
  void replaceRange(int start, int end, Iterable<StudyStroke> values) {
    _items.replaceRange(start, end, values);
    _invalidateFrom(start);
  }
}
