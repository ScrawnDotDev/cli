#!/usr/bin/env node
const { existsSync, mkdirSync, createWriteStream } = require("fs");
const { readFile, unlink } = require("fs/promises");
const { join } = require("path");
const { homedir } = require("os");
const { spawn } = require("child_process");
const { createGunzip } = require("zlib");
const { pipeline } = require("stream/promises");
const AdmZip = require("adm-zip");

const PKG = JSON.parse(
  require("fs").readFileSync(join(__dirname, "..", "package.json"), "utf-8")
);
const VERSION = PKG.version;
const REPO = "ScrawnDotDev/CLI";

const platformMap = {
  win32: "windows",
  darwin: "darwin",
  linux: "linux",
};

const archMap = {
  x64: "amd64",
  arm64: "arm64",
};

function binaryName() {
  const base = "scrawn";
  return process.platform === "win32" ? `${base}.exe` : base;
}

function archiveName() {
  const os = platformMap[process.platform];
  const arch = archMap[process.arch];
  if (!os || !arch) {
    console.error(`Unsupported platform: ${process.platform} ${process.arch}`);
    process.exit(1);
  }
  const ext = process.platform === "win32" ? "zip" : "tar.gz";
  return `scrawn_v${VERSION}_${os}_${arch}.${ext}`;
}

function cacheDir() {
  const dir = join(homedir(), ".scrawn", "bin");
  if (!existsSync(dir)) mkdirSync(dir, { recursive: true });
  return dir;
}

function cachedBinaryPath() {
  return join(cacheDir(), `scrawn-v${VERSION}${process.platform === "win32" ? ".exe" : ""}`);
}

async function download(url, dest) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`Download failed: ${res.status} ${res.statusText}`);
  const file = createWriteStream(dest);
  await pipeline(res.body, file);
}

async function extractTarGz(archive, destDir) {
  const { createReadStream } = require("fs");
  const { Extract } = require("tar");
  const gunzip = createGunzip();
  await pipeline(
    createReadStream(archive),
    gunzip,
    new Extract({ cwd: destDir, filter: (path) => path === binaryName() || path === `/${binaryName()}` })
  );
}

async function extractZip(archive, destDir) {
  const zip = new AdmZip(archive);
  const entry = zip.getEntry(binaryName());
  if (!entry) throw new Error(`Binary ${binaryName()} not found in archive`);
  zip.extractEntryTo(entry, destDir, false, true);
}

async function getBinary() {
  const cached = cachedBinaryPath();
  if (existsSync(cached)) return cached;

  const tmpDir = cacheDir();
  const arch = archiveName();
  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${arch}`;
  const tmpArchive = join(tmpDir, arch);

  process.stdout.write(`Downloading scrawn v${VERSION}...`);
  try {
    await download(url, tmpArchive);

    if (process.platform === "win32") {
      await extractZip(tmpArchive, tmpDir);
    } else {
      await extractTarGz(tmpArchive, tmpDir);
    }

    const extracted = join(tmpDir, binaryName());
    if (existsSync(extracted) && extracted !== cached) {
      require("fs").renameSync(extracted, cached);
    }

    try { await unlink(tmpArchive); } catch {}

    // Make executable on non-Windows
    if (process.platform !== "win32") {
      require("fs").chmodSync(cached, 0o755);
    }

    process.stdout.write(" done\n");
  } catch (err) {
    process.stdout.write("\n");
    console.error(`Failed to download scrawn: ${err.message}`);
    process.exit(1);
  }

  return cached;
}

async function main() {
  const binary = await getBinary();
  const args = process.argv.slice(2);

  const child = spawn(binary, args, { stdio: "inherit" });

  child.on("exit", (code) => process.exit(code ?? 1));
  child.on("error", (err) => {
    console.error(`Failed to run scrawn: ${err.message}`);
    process.exit(1);
  });
}

main();
