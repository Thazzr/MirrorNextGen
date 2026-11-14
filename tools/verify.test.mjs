import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { dirname, join } from 'node:path'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import {
  STEP_NAMES,
  exitCodeFor,
  formatSummary,
  parseArgs,
  planSteps,
  runGate,
  selftestCommand,
} from './verify.mjs'

const here = dirname(fileURLToPath(import.meta.url))

function plan(argv = [], env = {}) {
  const parsed = parseArgs(argv)
  assert.ok(parsed.opts, parsed.error)
  return { opts: parsed.opts, steps: planSteps(parsed.opts, env) }
}

/** Fake runner: records calls, fails the named steps. */
function fakeRunner(failing = []) {
  const calls = []
  const runStep = async (step) => {
    calls.push(step.name)
    // yield so the two strands really interleave
    await new Promise((r) => setTimeout(r, 1))
    return { ok: !failing.includes(step.name), output: `out ${step.name}` }
  }
  return { calls, runStep }
}

const byName = (results) => Object.fromEntries(results.map((r) => [r.name, r.result]))

test('all steps ok -> exit 0', async () => {
  const { steps } = plan()
  const { runStep } = fakeRunner()
  const { results, exitCode } = await runGate({ steps, runStep })
  assert.equal(exitCode, 0)
  assert.equal(results.length, STEP_NAMES.length)
  assert.ok(results.every((r) => r.result === 'ok'))
})

test('a failing step stops only its strand', async () => {
  const { steps } = plan()
  const { runStep, calls } = fakeRunner(['gofmt'])
  const { results, exitCode } = await runGate({ steps, runStep })
  const r = byName(results)
  assert.equal(exitCode, 1)
  assert.equal(r.gofmt, 'failed')
  assert.equal(r['go-vet'], 'not-run')
  assert.equal(r['go-test'], 'not-run')
  assert.equal(r['go-mod-tidy'], 'ok')
  for (const n of ['api-types-drift', 'typecheck', 'lint', 'vitest', 'build']) {
    assert.equal(r[n], 'ok', n)
  }
  assert.ok(!calls.includes('go-vet'))
})

test('a throwing runner counts as failed', async () => {
  const { steps } = plan(['--only', 'gofmt,lint'])
  const runStep = async (s) => {
    if (s.name === 'gofmt') throw new Error('boom')
    return { ok: true, output: '' }
  }
  const { results, exitCode } = await runGate({ steps, runStep })
  assert.equal(exitCode, 1)
  assert.equal(byName(results).gofmt, 'failed')
  assert.equal(byName(results).lint, 'ok')
})

test('skipped counts as success; --skip-build skips build', async () => {
  const { steps } = plan(['--skip-build'])
  const { runStep, calls } = fakeRunner()
  const { results, exitCode } = await runGate({ steps, runStep })
  assert.equal(byName(results).build, 'skipped')
  assert.ok(!calls.includes('build'))
  assert.equal(exitCode, 0)
  assert.equal(exitCodeFor([{ result: 'skipped' }, { result: 'ok' }]), 0)
  assert.equal(exitCodeFor([{ result: 'ok' }, { result: 'not-run' }]), 1)
})

test('unknown --only name is an error naming the culprit', () => {
  const parsed = parseArgs(['--only', 'gofmt,nope'])
  assert.match(parsed.error, /nope/)
  assert.equal(parsed.opts, undefined)
  assert.ok(parseArgs(['--bogus']).error)
})

test('--only runs exactly the named steps (comma-separated and repeated)', async () => {
  for (const argv of [['--only', 'gofmt,lint'], ['--only', 'gofmt', '--only', 'lint']]) {
    const { steps } = plan(argv)
    const { runStep, calls } = fakeRunner()
    const { results } = await runGate({ steps, runStep })
    assert.deepEqual(calls.sort(), ['gofmt', 'lint'])
    assert.deepEqual(results.map((r) => r.name), ['gofmt', 'lint'])
  }
})

test('--sequential keeps the order and never overlaps', async () => {
  const { steps } = plan(['--sequential'])
  const { runStep, calls } = fakeRunner()
  await runGate({ steps, runStep, sequential: true })
  assert.deepEqual(calls, STEP_NAMES)
})

test('failing selftest: strands never start, exit 1', async () => {
  const { steps } = plan()
  const { runStep, calls } = fakeRunner(['selftest'])
  const { results, exitCode } = await runGate({ steps, runStep })
  assert.equal(exitCode, 1)
  assert.deepEqual(calls, ['selftest'])
  assert.equal(byName(results).selftest, 'failed')
  assert.equal(byName(results).gofmt, 'not-run')
  assert.equal(byName(results).lint, 'not-run')
})

test('VERIFY_SELFTEST_RUNNING=1: selftest is skipped and not executed', async () => {
  const { steps } = plan([], { VERIFY_SELFTEST_RUNNING: '1' })
  const { runStep, calls } = fakeRunner()
  const { results, exitCode } = await runGate({ steps, runStep })
  assert.equal(byName(results).selftest, 'skipped')
  assert.ok(!calls.includes('selftest'))
  assert.equal(exitCode, 0)
})

test('summary lists every step with its result', () => {
  const text = formatSummary([
    { name: 'gofmt', strand: 'backend', ms: 1500, result: 'ok' },
    { name: 'lint', strand: 'frontend', ms: 0, result: 'not-run' },
  ])
  assert.match(text, /gofmt\s+backend\s+1\.5s\s+ok/)
  assert.match(text, /lint\s+frontend\s+0\.0s\s+not-run/)
})

test('end to end: an unknown --only step exits 2 before anything runs', () => {
  const r = spawnSync(process.execPath, [join(here, 'verify.mjs'), '--only', 'nope'], {
    encoding: 'utf8',
  })
  assert.equal(r.status, 2)
  assert.match(r.stderr, /valid steps:/)
})

test('selftest child gets the recursion-guard env marker and the test glob', () => {
  const { cmd, args, env } = selftestCommand()
  assert.equal(cmd, 'node')
  assert.deepEqual(args, ['--test', 'tools/*.test.mjs'])
  assert.equal(env.VERIFY_SELFTEST_RUNNING, '1')
})
