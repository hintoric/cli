#!/usr/bin/env node
// Wrapper for the @hintoric/cli npm distribution.
//
// At install time, npm picks exactly one of the optional platform packages
// (@hintoric/cli-<platform>-<arch>) based on its `os` / `cpu` fields. This
// wrapper resolves that package at runtime and execs its `hint` binary,
// passing through stdio and exit code.
//
// Why optionalDependencies + require.resolve instead of a postinstall
// download: npm install --ignore-scripts (default in many CI sandboxes)
// breaks postinstall-download wrappers but does not affect this pattern,
// because the binary arrived as a regular package payload.

'use strict';

const { spawnSync } = require('node:child_process');
const path = require('node:path');

const PACKAGES = Object.freeze({
  'darwin-arm64': '@hintoric/cli-darwin-arm64',
  'darwin-x64':   '@hintoric/cli-darwin-x64',
  'linux-arm64':  '@hintoric/cli-linux-arm64',
  'linux-x64':    '@hintoric/cli-linux-x64',
  'win32-x64':    '@hintoric/cli-windows-x64',
});

function platformKey() {
  return `${process.platform}-${process.arch}`;
}

function resolveBinary() {
  const key = platformKey();
  const pkg = PACKAGES[key];
  if (!pkg) {
    throw new Error(
      `hint: no prebuilt binary for ${key}. ` +
      `Supported: ${Object.keys(PACKAGES).join(', ')}. ` +
      `Build from source: https://github.com/hintoric/cli`
    );
  }
  let pkgJsonPath;
  try {
    pkgJsonPath = require.resolve(`${pkg}/package.json`);
  } catch (_err) {
    throw new Error(
      `hint: ${pkg} is not installed. ` +
      `npm may have skipped optional dependencies. ` +
      `Reinstall with: npm install --include=optional @hintoric/cli`
    );
  }
  const ext = process.platform === 'win32' ? '.exe' : '';
  return path.join(path.dirname(pkgJsonPath), `hint${ext}`);
}

function main() {
  let bin;
  try {
    bin = resolveBinary();
  } catch (err) {
    console.error(err.message);
    process.exit(1);
  }
  const result = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });
  if (result.error) {
    console.error('hint:', result.error.message);
    process.exit(1);
  }
  process.exit(result.status ?? 1);
}

if (require.main === module) {
  main();
}

module.exports = { PACKAGES, platformKey, resolveBinary };
