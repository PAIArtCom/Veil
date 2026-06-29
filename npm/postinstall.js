#!/usr/bin/env node
"use strict";

// Downloads the correct Veil binary for the current platform at npm install time.
// No external dependencies — uses only Node.js built-ins.

const https = require("https");
const http = require("http");
const fs = require("fs");
const path = require("path");
const crypto = require("crypto");

function client(url) {
  return url.startsWith("http://") ? http : https;
}

const REPO = "PAIArtCom/Veil";
const VERSION = require("./package.json").version;
const TAG = `v${VERSION}`;
const RELEASES = process.env.VEIL_DOWNLOAD_BASE || `https://github.com/${REPO}/releases/download/${TAG}`;
const BIN_DIR = path.join(__dirname, "bin");

// platform map
const PLATFORM = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
}[process.platform];

const ARCH = {
  x64: "amd64",
  arm64: "arm64",
}[process.arch];

if (!PLATFORM || !ARCH) {
  console.error(`veil: unsupported platform ${process.platform}/${process.arch}`);
  process.exit(1);
}

const EXE = PLATFORM === "windows" ? ".exe" : "";
const ARTIFACT = `veil-${TAG}-${PLATFORM}-${ARCH}${EXE}`;
const DEST = path.join(BIN_DIR, `veil-bin${EXE}`);

// helpers
function download(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (u) => {
      client(u).get(u, (res) => {
        if (res.statusCode === 301 || res.statusCode === 302) {
          return follow(res.headers.location);
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`HTTP ${res.statusCode} for ${u}`));
        }
        const out = fs.createWriteStream(dest);
        res.pipe(out);
        out.on("finish", resolve);
        out.on("error", reject);
      }).on("error", reject);
    };
    follow(url);
  });
}

function fetchText(url) {
  return new Promise((resolve, reject) => {
    const follow = (u) => {
      client(u).get(u, (res) => {
        if (res.statusCode === 301 || res.statusCode === 302) {
          return follow(res.headers.location);
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`HTTP ${res.statusCode} for ${u}`));
        }
        let body = "";
        res.on("data", (chunk) => (body += chunk));
        res.on("end", () => resolve(body));
      }).on("error", reject);
    };
    follow(url);
  });
}

function sha256(file) {
  return crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
}

// main
(async () => {
  fs.mkdirSync(BIN_DIR, { recursive: true });

  console.log(`veil: downloading ${ARTIFACT}...`);
  await download(`${RELEASES}/${ARTIFACT}`, DEST);

  const checksums = await fetchText(`${RELEASES}/checksums.txt`);
  const line = checksums.split("\n").find((l) => l.trim().split(/\s+/)[1] === ARTIFACT);
  if (!line) throw new Error(`no checksum entry for ${ARTIFACT}`);
  const expected = line.trim().split(/\s+/)[0];
  const actual = sha256(DEST);
  if (actual !== expected) {
    fs.unlinkSync(DEST);
    throw new Error(`checksum mismatch\n  expected: ${expected}\n  got:      ${actual}`);
  }

  if (PLATFORM !== "windows") fs.chmodSync(DEST, 0o755);

  console.log(`veil: installed ${TAG} (${PLATFORM}/${ARCH})`);
})().catch((err) => {
  console.error(`veil: install failed — ${err.message}`);
  console.error(`      Download manually: https://github.com/${REPO}/releases/tag/${TAG}`);
  process.exit(1);
});
