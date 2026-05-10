#!/usr/bin/env node
// Build per-platform npm packages + the meta package from a goreleaser run.
//
// Usage:
//   node npm/scripts/pack.mjs --version 0.1.2 --dist dist
//
// Reads <dist>/artifacts.json (goreleaser's machine-readable manifest),
// finds every Binary artifact for `hint`, and writes a publishable tree at
// npm/dist/<pkg>/ for each, plus the meta package npm/dist/cli/.
//
// The output is what goes to `npm publish`. Nothing in npm/dist/ should be
// committed; the workflow regenerates it per release.

import { mkdir, copyFile, writeFile, readFile, rm, chmod } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const NPM_ROOT = path.resolve(HERE, '..');
const REPO_ROOT = path.resolve(NPM_ROOT, '..');

// goreleaser goos/goarch → npm os/cpu fields and our package suffix.
// process.platform reports 'win32' for windows, but npm's `os` field also
// uses 'win32', so they line up. process.arch reports 'x64' for amd64;
// npm's `cpu` field uses 'x64' too.
const PLATFORMS = {
  'darwin/amd64':  { suffix: 'darwin-x64',     os: ['darwin'], cpu: ['x64']   },
  'darwin/arm64':  { suffix: 'darwin-arm64',   os: ['darwin'], cpu: ['arm64'] },
  'linux/amd64':   { suffix: 'linux-x64',      os: ['linux'],  cpu: ['x64']   },
  'linux/arm64':   { suffix: 'linux-arm64',    os: ['linux'],  cpu: ['arm64'] },
  'windows/amd64': { suffix: 'windows-x64',    os: ['win32'],  cpu: ['x64']   },
};

function parseArgs(argv) {
  const out = { version: null, dist: 'dist' };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--version') out.version = argv[++i];
    else if (a === '--dist') out.dist = argv[++i];
    else if (a === '--help' || a === '-h') {
      console.log('node npm/scripts/pack.mjs --version <semver> [--dist dist]');
      process.exit(0);
    } else {
      throw new Error(`unknown arg: ${a}`);
    }
  }
  if (!out.version) throw new Error('--version is required');
  // npm doesn't allow leading "v" — strip if the caller passed a tag name.
  if (out.version.startsWith('v')) out.version = out.version.slice(1);
  return out;
}

async function readArtifacts(distDir) {
  const p = path.join(distDir, 'artifacts.json');
  if (!existsSync(p)) {
    throw new Error(`${p} not found — run goreleaser first`);
  }
  return JSON.parse(await readFile(p, 'utf8'));
}

function pickHintBinaries(artifacts) {
  // Goreleaser tags Binary artifacts with goos/goarch and a `name` of the
  // executable. We only ship `hint` (not `hint-build`), and only for the
  // platforms enumerated in PLATFORMS.
  const out = [];
  for (const a of artifacts) {
    if (a.type !== 'Binary') continue;
    if (a.name !== 'hint' && a.name !== 'hint.exe') continue;
    const key = `${a.goos}/${a.goarch}`;
    const plat = PLATFORMS[key];
    if (!plat) continue;
    out.push({ artifact: a, platform: plat });
  }
  return out;
}

async function writePlatformPackage(outDir, version, plat, srcBinary, isWindows) {
  await mkdir(outDir, { recursive: true });
  const binName = isWindows ? 'hint.exe' : 'hint';
  const dst = path.join(outDir, binName);
  await copyFile(srcBinary, dst);
  // 0o755 keeps the executable bit on Unix; npm preserves file modes inside
  // the package tarball so end users get an executable, not a 0644 file
  // they have to chmod.
  await chmod(dst, 0o755);
  const pkg = {
    name: `@hintoric/cli-${plat.suffix}`,
    version,
    description: `${plat.suffix} prebuilt binary for @hintoric/cli`,
    homepage: 'https://github.com/hintoric/cli',
    license: 'MIT',
    repository: {
      type: 'git',
      url: 'git+https://github.com/hintoric/cli.git',
    },
    files: [binName],
    os: plat.os,
    cpu: plat.cpu,
  };
  await writeFile(
    path.join(outDir, 'package.json'),
    JSON.stringify(pkg, null, 2) + '\n',
  );
}

async function writeMetaPackage(outDir, version, suffixes) {
  await mkdir(path.join(outDir, 'bin'), { recursive: true });
  // Copy the wrapper + README from the committed skeleton. The skeleton's
  // version + optionalDependencies versions are 0.0.0-development; we
  // overwrite them with the real release version.
  await copyFile(
    path.join(NPM_ROOT, 'cli', 'bin', 'hint.js'),
    path.join(outDir, 'bin', 'hint.js'),
  );
  await chmod(path.join(outDir, 'bin', 'hint.js'), 0o755);
  await copyFile(
    path.join(NPM_ROOT, 'cli', 'README.md'),
    path.join(outDir, 'README.md'),
  );
  const skeleton = JSON.parse(
    await readFile(path.join(NPM_ROOT, 'cli', 'package.json'), 'utf8'),
  );
  skeleton.version = version;
  skeleton.optionalDependencies = Object.fromEntries(
    suffixes.map((s) => [`@hintoric/cli-${s}`, version]),
  );
  // Drop dev-only fields that don't belong in a published package.
  delete skeleton.scripts;
  await writeFile(
    path.join(outDir, 'package.json'),
    JSON.stringify(skeleton, null, 2) + '\n',
  );
}

async function main() {
  const { version, dist } = parseArgs(process.argv.slice(2));
  const distDir = path.resolve(REPO_ROOT, dist);
  const outRoot = path.resolve(NPM_ROOT, 'dist');

  // Fresh slate every run — leftover dirs from a previous version would
  // get re-published with the new tag and confuse npm.
  if (existsSync(outRoot)) await rm(outRoot, { recursive: true, force: true });

  const artifacts = await readArtifacts(distDir);
  const hits = pickHintBinaries(artifacts);
  if (hits.length === 0) {
    throw new Error(`no hint Binary artifacts found in ${distDir}/artifacts.json`);
  }

  const suffixes = [];
  for (const { artifact, platform } of hits) {
    const outDir = path.join(outRoot, `cli-${platform.suffix}`);
    const isWindows = artifact.goos === 'windows';
    const srcBinary = path.resolve(REPO_ROOT, artifact.path);
    await writePlatformPackage(outDir, version, platform, srcBinary, isWindows);
    console.log(`packed ${platform.suffix} → ${path.relative(REPO_ROOT, outDir)}`);
    suffixes.push(platform.suffix);
  }

  // Refuse to ship a meta package whose optionalDependencies mention
  // platform packages we did not actually build — npm install would then
  // fail to resolve the missing dep on those platforms.
  const expected = new Set(Object.values(PLATFORMS).map((p) => p.suffix));
  const got = new Set(suffixes);
  for (const s of expected) {
    if (!got.has(s)) {
      throw new Error(
        `goreleaser dist is missing platform ${s} — refusing to publish a ` +
        `meta package whose optionalDependencies include a non-existent ` +
        `release. Re-run goreleaser to produce all 5 platforms.`,
      );
    }
  }

  const metaOut = path.join(outRoot, 'cli');
  await writeMetaPackage(metaOut, version, suffixes);
  console.log(`packed meta → ${path.relative(REPO_ROOT, metaOut)}`);
  console.log(`done. ${suffixes.length} platform package(s) + meta at ${path.relative(REPO_ROOT, outRoot)}/`);
}

main().catch((err) => {
  console.error('pack:', err.message);
  process.exit(1);
});
