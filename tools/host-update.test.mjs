import assert from 'node:assert/strict'
import { spawn, spawnSync } from 'node:child_process'
import { chmodSync, readdirSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { delimiter, dirname, join, resolve } from 'node:path'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const script = join(root, 'deploy', 'host', 'mirror-update.sh')
const installScript = join(root, 'deploy', 'host', 'install.sh')

const bash = spawnSync('bash', ['-c', 'command -v flock >/dev/null && echo ok'], { encoding: 'utf8' })
const skip = process.platform === 'win32' || bash.stdout.trim() !== 'ok'
  ? 'bash with flock is not available (e.g. Windows)'
  : false

// Builds a temp mirror dir plus a fake docker that records its arguments.
function setup({ dockerExit = 0, dockerOutput = '', sleep = 0 } = {}) {
  const dir = mkdtempSync(join(tmpdir(), 'host-update-'))
  const mirror = join(dir, 'mirror')
  const bin = join(dir, 'bin')
  mkdirSync(join(mirror, 'control'), { recursive: true })
  mkdirSync(bin)
  writeFileSync(join(mirror, 'compose.yaml'), 'name: x\n')
  writeFileSync(join(mirror, '.env'), 'TZ=UTC\n')
  writeFileSync(join(mirror, 'control', 'update-request'), 'x')
  writeFileSync(join(dir, 'output.txt'), dockerOutput)
  const calls = join(dir, 'calls.txt')
  // Only the "up" call fails, so the pull is seen to run first.
  writeFileSync(join(bin, 'docker'), `#!/usr/bin/env bash
printf '%s\\n' "$*" >> "${calls}"
sleep ${sleep}
cat "${join(dir, 'output.txt')}"
case "$*" in *" up "*) exit ${dockerExit} ;; esac
exit 0
`)
  chmodSync(join(bin, 'docker'), 0o755)
  const env = { ...process.env, MIRROR_DIR: mirror, PATH: `${bin}${delimiter}${process.env.PATH}` }
  const readCalls = () => (existsSync(calls) ? readFileSync(calls, 'utf8').trim().split('\n') : [])
  const status = () => JSON.parse(readFileSync(join(mirror, 'control', 'update-status.json'), 'utf8'))
  return { dir, mirror, env, readCalls, status, cleanup: () => rmSync(dir, { recursive: true, force: true }) }
}

const run = (t) => spawnSync('bash', [script], { env: t.env, encoding: 'utf8' })

test('success: exact docker calls, status succeeded, request removed', { skip }, () => {
  const t = setup()
  try {
    const r = run(t)
    assert.equal(r.status, 0, r.stderr)
    const common = `compose --project-directory ${t.mirror} -f ${t.mirror}/compose.yaml --env-file ${t.mirror}/.env`
    assert.deepEqual(t.readCalls(), [`${common} pull`, `${common} up -d --remove-orphans`])
    const s = t.status()
    assert.equal(s.state, 'succeeded')
    assert.equal(s.message, 'Aktualisiert.')
    const iso = /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d[+-]\d\d:\d\d$/
    assert.match(s.startedAt, iso)
    assert.match(s.finishedAt, iso)
    assert.equal(existsSync(join(t.mirror, 'control', 'update-request')), false)
    assert.equal(statSync(join(t.mirror, 'control', 'update-status.json')).mode & 0o777, 0o644)
  } finally {
    t.cleanup()
  }
})

test('failure: status failed with escaped last 5 log lines, request removed', { skip }, () => {
  const lines = ['l1', 'l2', 'l3', 'l4 "quoted"', 'l5 back\\slash', 'l6\ttab', 'l7']
  const t = setup({ dockerExit: 1, dockerOutput: lines.join('\n') + '\n' })
  try {
    const r = run(t)
    assert.notEqual(r.status, 0)
    const raw = readFileSync(join(t.mirror, 'control', 'update-status.json'), 'utf8')
    assert.ok(raw.includes('\\"quoted\\"') && raw.includes('\\\\slash') && raw.includes('\\t'))
    const s = JSON.parse(raw)
    assert.equal(s.state, 'failed')
    assert.ok(s.finishedAt && s.startedAt)
    // The fake prints the 7 lines for pull and for up; the last 5 log lines are l3..l7.
    assert.deepEqual(s.message.split('\n'), ['l3', 'l4 "quoted"', 'l5 back\\slash', 'l6\ttab', 'l7'])
    assert.equal(existsSync(join(t.mirror, 'control', 'update-request')), false)
    assert.equal(t.readCalls().length, 2)
  } finally {
    t.cleanup()
  }
})

test('running status is written before docker finishes', { skip }, async () => {
  const t = setup({ sleep: 1 })
  try {
    const child = spawn('bash', [script], { env: t.env })
    const done = new Promise((res) => child.on('close', res))
    const deadline = Date.now() + 5000
    while (!existsSync(join(t.mirror, 'control', 'update-status.json')) && Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 20))
    }
    const s = t.status()
    assert.equal(s.state, 'running')
    assert.ok(s.startedAt)
    assert.equal(s.finishedAt, undefined)
    const statusFile = join(t.mirror, 'control', 'update-status.json')
    const inodeRunning = statSync(statusFile).ino
    assert.equal(await done, 0)
    assert.equal(t.status().state, 'succeeded')
    // Replaced by rename (new inode), not rewritten in place, and no temp file left behind.
    assert.notEqual(statSync(statusFile).ino, inodeRunning)
    assert.deepEqual(readdirSync(join(t.mirror, 'control')).filter((f) => f.startsWith('.update-status.')), [])
  } finally {
    t.cleanup()
  }
})

test('lock: a second run while the first is active exits 0 without calling docker', { skip }, async () => {
  const t = setup({ sleep: 2 })
  try {
    const child = spawn('bash', [script], { env: t.env })
    const done = new Promise((res) => child.on('close', res))
    const deadline = Date.now() + 5000
    while (t.readCalls().length === 0 && Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 20))
    }
    const second = run(t)
    assert.equal(second.status, 0, second.stderr)
    assert.equal(t.readCalls().length, 1, 'only the first run called docker (pull in progress)')
    assert.equal(await done, 0)
    assert.equal(t.readCalls().length, 2)
  } finally {
    t.cleanup()
  }
})

test('missing MIRROR_DIR fails', { skip }, () => {
  const env = { ...process.env }
  delete env.MIRROR_DIR
  const r = spawnSync('bash', [script], { env, encoding: 'utf8' })
  assert.notEqual(r.status, 0)
  assert.match(r.stderr, /MIRROR_DIR/)
})

test('install.sh render_unit substitutes the mirror directory', { skip }, () => {
  const unit = join(root, 'deploy', 'host', 'mirror-update.service')
  const r = spawnSync('bash', ['-c', `source "${installScript}"; render_unit "${unit}" '/opt/my mirror'`], { encoding: 'utf8' })
  assert.equal(r.status, 0, r.stderr)
  assert.ok(r.stdout.includes('Environment=MIRROR_DIR=/opt/my mirror'))
  assert.ok(r.stdout.includes('ExecStart=/opt/my mirror/host/mirror-update.sh'))
  assert.ok(!r.stdout.includes('@MIRROR_DIR@'))
})

test('install.sh render_unit replaces every occurrence, also several per line', { skip }, () => {
  const dir = mkdtempSync(join(tmpdir(), 'render-unit-'))
  try {
    const unit = join(dir, 'u.service')
    writeFileSync(unit, 'X=@MIRROR_DIR@:@MIRROR_DIR@\nY=@MIRROR_DIR@\n')
    const r = spawnSync('bash', ['-c', `source "${installScript}"; render_unit "${unit}" /a`], { encoding: 'utf8' })
    assert.equal(r.status, 0, r.stderr)
    assert.equal(r.stdout, 'X=/a:/a\nY=/a\n')
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})

test('install.sh refuses to run without root', { skip: skip || process.getuid?.() === 0 }, () => {
  const r = spawnSync('bash', [installScript], { encoding: 'utf8' })
  assert.equal(r.status, 1)
  assert.match(r.stderr, /sudo/)
})
