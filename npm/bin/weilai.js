#!/usr/bin/env node
// 按平台挑 vendor 里的 Go 二进制执行（WorkBuddy 用托管 Node 运行时 npm install -g 安装，bin 目录自动进 PATH）。
'use strict';
const path = require('path');
const fs = require('fs');
const { spawnSync } = require('child_process');
const os = { darwin: 'darwin', linux: 'linux', win32: 'windows' }[process.platform];
const arch = { arm64: 'arm64', x64: 'amd64' }[process.arch];
const exe = path.join(__dirname, '..', 'vendor', `weilai-${os}-${arch}${os === 'windows' ? '.exe' : ''}`);
if (!os || !arch || !fs.existsSync(exe)) {
  console.error(`weilai: 暂不支持 ${process.platform}-${process.arch}`);
  process.exit(2);
}
try { fs.chmodSync(exe, 0o755); } catch (e) { /* Windows 或只读目录：忽略 */ }
const r = spawnSync(exe, process.argv.slice(2), { stdio: 'inherit' });
process.exit(r.status === null ? 1 : r.status);
