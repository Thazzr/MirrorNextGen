import assert from 'node:assert/strict'
import { test } from 'node:test'
import { cliffArgs, parseArgs } from './changelog.mjs'

test('no flags is valid', () => {
  assert.deepEqual(parseArgs([]).opts, { tag: null, message: null, withCommit: null })
})

test('tag with message is valid', () => {
  const { opts } = parseArgs(['--tag', 'v0.2.0', '--message', 'Phase 1'])
  assert.equal(opts.tag, 'v0.2.0')
  assert.equal(opts.message, 'Phase 1')
})

test('--with-commit alone and combined with a tag', () => {
  assert.equal(parseArgs(['--with-commit', 'feat(x): y']).opts.withCommit, 'feat(x): y')
  assert.ok(parseArgs(['--tag', 'v1.0.0', '--message', 'm', '--with-commit', 's']).opts)
})

test('tag without message is rejected, and message without tag', () => {
  assert.match(parseArgs(['--tag', 'v0.2.0']).error, /--message/)
  assert.match(parseArgs(['--message', 'm']).error, /--tag/)
})

test('tag format is checked', () => {
  for (const bad of ['0.2.0', 'v0.2', 'v1.2.3-rc1', 'vx.y.z']) {
    assert.match(parseArgs(['--tag', bad, '--message', 'm']).error, /tag must look like/, bad)
  }
})

test('unknown flag and missing value are rejected', () => {
  assert.match(parseArgs(['--nope']).error, /unknown argument/)
  assert.match(parseArgs(['--tag']).error, /needs a value/)
})

test('cliff args cover the full history and add --with-commit on demand', () => {
  assert.deepEqual(cliffArgs({}), ['-c', 'cliff.toml', '-o', 'CHANGELOG.md'])
  assert.deepEqual(cliffArgs({ withCommit: 's' }).slice(-2), ['--with-commit', 's'])
})
