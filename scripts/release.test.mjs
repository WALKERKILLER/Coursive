import assert from 'node:assert/strict';
import { test } from 'node:test';
import { ReleaseError, compare, nextPrerelease, promotion } from './release.mjs';

const tags = ['v0.1.0-beta.1', 'v0.1.0-beta.2', 'v0.1.0-beta.10', 'v0.1.0-beta.9'];

test('versions follow semantic-versioning precedence', () => {
  assert.ok(compare('0.1.0-beta.10', '0.1.0-beta.9') > 0);
  assert.ok(compare('0.1.0', '0.1.0-beta.10') > 0);
  assert.ok(compare('0.1.0-rc.1', '0.1.0-beta.2') > 0);
  assert.ok(compare('0.10.0', '0.9.1') > 0);
  assert.equal(compare('1.2.3', '1.2.3'), 0);
});

test('a pre-release continues the newest one and numbers its build after every tag', () => {
  assert.deepEqual(nextPrerelease(tags), { tag: 'v0.1.0-beta.11', version: '0.1.0-beta.11', build: 5 });
  assert.deepEqual(nextPrerelease(tags, 'v0.2.0-beta.1'), { tag: 'v0.2.0-beta.1', version: '0.2.0-beta.1', build: 5 });
});

test('a pre-release is refused when it exists, is stable or follows its release', () => {
  assert.throws(() => nextPrerelease(tags, '0.1.0-beta.2'), /already exists/);
  assert.throws(() => nextPrerelease(tags, '0.2.0'), /not a pre-release/);
  assert.throws(() => nextPrerelease([...tags, 'v0.1.0'], '0.1.0-beta.12'), /already released/);
  assert.throws(() => nextPrerelease([...tags, 'v0.1.0']), /name the next version, for example 0\.2\.0-beta\.1/);
  assert.throws(() => nextPrerelease([]), ReleaseError);
});

test('a promotion releases the stable version of the newest pre-release on the commit', () => {
  assert.deepEqual(promotion(['v0.1.0-beta.9', 'v0.1.0-beta.10'], tags),
    { tag: 'v0.1.0', version: '0.1.0', build: 5, prerelease: 'v0.1.0-beta.10', previous: '' });
  assert.equal(promotion(['v0.2.0-rc.1'], [...tags, 'v0.1.0', 'v0.2.0-rc.1', 'v1.0.0']).previous, 'v0.1.0');
});

test('a promotion is refused without a pre-release or after its release', () => {
  assert.throws(() => promotion([], tags), /no pre-release tag/);
  assert.throws(() => promotion(['v0.1.0'], [...tags, 'v0.1.0']), /no pre-release tag/);
  assert.throws(() => promotion(['v0.1.0-beta.10'], [...tags, 'v0.1.0']), /already released/);
});
