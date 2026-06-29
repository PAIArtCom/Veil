#!/usr/bin/env node
"use strict";

// Thin wrapper — spawns the downloaded Veil binary and forwards all I/O.

const { spawnSync } = require("child_process");
const path = require("path");

const ext = process.platform === "win32" ? ".exe" : "";
const bin = path.join(__dirname, `veil-bin${ext}`);

const result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
process.exit(result.status ?? 1);
