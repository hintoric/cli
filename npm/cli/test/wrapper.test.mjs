// Smoke tests for the wrapper. We cannot exercise the spawn path without a
// real binary installed, so we test the lookup logic and the platform map
// shape — which is what drifts when goreleaser's build matrix changes.

import test from 'node:test';
import assert from 'node:assert/strict';

import { PACKAGES, platformKey } from '../bin/hint.js';

test('PACKAGES covers exactly the goreleaser build matrix', () => {
  // .goreleaser.yaml builds darwin amd64+arm64 (CGO), linux amd64+arm64,
  // windows amd64. windows/arm64 is excluded. Drift between this set and
  // the wrapper means an installed user falls through to "no binary for
  // <key>" — catch it in CI before publish.
  const expected = [
    'darwin-arm64',
    'darwin-x64',
    'linux-arm64',
    'linux-x64',
    'win32-x64',
  ].sort();
  assert.deepEqual(Object.keys(PACKAGES).sort(), expected);
});

test('every PACKAGES entry is under the @hintoric scope', () => {
  for (const [key, pkg] of Object.entries(PACKAGES)) {
    assert.match(pkg, /^@hintoric\/cli-/, `${key} -> ${pkg}`);
  }
});

test('platformKey reports process.platform-process.arch', () => {
  assert.equal(platformKey(), `${process.platform}-${process.arch}`);
});

test('PACKAGES is frozen — runtime mutation would silently break installs', () => {
  assert.throws(() => {
    PACKAGES['fake'] = 'oops';
  });
});
