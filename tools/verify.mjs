// The verification gate: `npm run verify`. Node >= 22, no dependencies, runs
// on Windows, Linux and macOS. See CLAUDE.md -> "Verifikation".
import { spawn } from 'node:child_process'
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const isWin = process.platform === 'win32'

export const SELFTEST = 'selftest'
export const BACKEND = 'backend'
export const FRONTEND = 'frontend'

/** Step names in execution order; the selftest runs before the strands. */
export const STEP_DEFS = [
  { name: SELFTEST, strand: null },
  { name: 'go-mod-tidy', strand: BACKEND },
  { name: 'go-generate-drift', strand: BACKEND },
  { name: 'gofmt', strand: BACKEND },
  { name: 'go-vet', strand: BACKEND },
  { name: 'go-test', strand: BACKEND },
  { name: 'api-types-drift', strand: FRONTEND },
  { name: 'typecheck', strand: FRONTEND },
  { name: 'lint', strand: FRONTEND },
  { name: 'vitest', strand: FRONTEND },
  { name: 'build', strand: FRONTEND },
]

export const STEP_NAMES = STEP_DEFS.map((s) => s.name)

/** Parses CLI args. Never exits; returns `{ error }` for invalid input. */
export function parseArgs(argv) {
  const opts = { sequential: false, skipBuild: false, race: false, only: null }
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]
    if (a === '--sequential') opts.sequential = true
    else if (a === '--skip-build') opts.skipBuild = true
    else if (a === '--race') opts.race = true
    else if (a === '--only' || a.startsWith('--only=')) {
      let value = a.startsWith('--only=') ? a.slice('--only='.length) : argv[++i]
      if (value === undefined) return { error: '--only needs a step name' }
      opts.only ??= []
      opts.only.push(...value.split(',').map((s) => s.trim()).filter(Boolean))
    } else return { error: `unknown argument: ${a}` }
  }
  const unknown = (opts.only ?? []).filter((n) => !STEP_NAMES.includes(n))
  if (unknown.length > 0) {
    return { error: `unknown step: ${unknown.join(', ')}` }
  }
  return { opts }
}

/** Picks the steps to run, in order, with the reason for skipping if any. */
export function planSteps(opts, env = process.env) {
  const only = opts.only ? new Set(opts.only) : null
  return STEP_DEFS.filter((s) => !only || only.has(s.name)).map((s) => {
    let skip = null
    if (s.name === SELFTEST && env.VERIFY_SELFTEST_RUNNING === '1') {
      skip = 'recursion guard (VERIFY_SELFTEST_RUNNING)'
    } else if (s.name === 'build' && opts.skipBuild) {
      skip = '--skip-build'
    }
    return { ...s, skip }
  })
}

/**
 * Runs the planned steps. `runStep(step)` resolves `{ ok, output }`.
 * Returns `{ results, exitCode }`; each result is
 * `{ name, strand, ms, result: ok|failed|skipped|not-run, output }`.
 */
export async function runGate({ steps, runStep, sequential = false }) {
  const results = []
  const record = (step, result, ms = 0, output = '') => {
    const r = { name: step.name, strand: step.strand ?? 'pre', ms, result, output }
    results.push(r)
    return r
  }

  async function exec(step) {
    if (step.skip) return record(step, 'skipped', 0, `skipped: ${step.skip}\n`)
    const t0 = Date.now()
    try {
      const { ok, output } = await runStep(step)
      return record(step, ok ? 'ok' : 'failed', Date.now() - t0, output ?? '')
    } catch (e) {
      return record(step, 'failed', Date.now() - t0, `${e?.stack ?? e}\n`)
    }
  }

  async function runList(list) {
    let broken = false
    for (const step of list) {
      if (broken) record(step, 'not-run')
      else if ((await exec(step)).result === 'failed') broken = true
    }
  }

  const pre = steps.filter((s) => s.strand === null)
  const strands = [BACKEND, FRONTEND].map((n) => steps.filter((s) => s.strand === n))

  // A failing selftest means the gate's own logic cannot be trusted: stop.
  await runList(pre)
  if (results.some((r) => r.result === 'failed')) {
    for (const step of strands.flat()) record(step, 'not-run')
  } else if (sequential) {
    for (const list of strands) await runList(list)
  } else {
    await Promise.all(strands.map(runList))
  }

  const order = (r) => STEP_NAMES.indexOf(r.name)
  results.sort((a, b) => order(a) - order(b))
  return { results, exitCode: exitCodeFor(results) }
}

export function exitCodeFor(results) {
  return results.every((r) => r.result === 'ok' || r.result === 'skipped') ? 0 : 1
}

export function formatSummary(results) {
  const rows = [['step', 'strand', 'duration', 'result']]
  for (const r of results) {
    rows.push([r.name, r.strand, `${(r.ms / 1000).toFixed(1)}s`, r.result])
  }
  const widths = rows[0].map((_, c) => Math.max(...rows.map((row) => row[c].length)))
  return rows
    .map((row) => row.map((cell, c) => cell.padEnd(widths[c])).join('  ').trimEnd())
    .join('\n')
}

function logFor(results, strand) {
  return results
    .filter((r) => r.strand === strand)
    .map((r) => `--- ${r.name} [${r.result}] ---\n${r.output}`)
    .join('\n')
}

// --- real step execution ---------------------------------------------------

function run(cmd, args, cwd, env = {}) {
  return new Promise((res) => {
    let output = ''
    // npm/npx are .cmd shims on Windows and need a shell there.
    const child = spawn(cmd, args, {
      cwd,
      env: { ...process.env, ...env },
      shell: isWin && (cmd === 'npm' || cmd === 'npx'),
    })
    child.stdout.on('data', (d) => (output += d))
    child.stderr.on('data', (d) => (output += d))
    child.on('error', (e) => res({ code: -1, output: output + `${e.message}\n` }))
    child.on('close', (code) => res({ code: code ?? -1, output }))
  })
}

async function runChecked(cmd, args, cwd, env) {
  const { code, output } = await run(cmd, args, cwd, env)
  return { ok: code === 0, output: `$ ${cmd} ${args.join(' ')}\n${output}` }
}

function driftCheck(generatedPath, committedPath, output) {
  const same = readFileSync(generatedPath).equals(readFileSync(committedPath))
  if (same) return { ok: true, output }
  return {
    ok: false,
    output: `${output}${committedPath} differs from what the contract generates.\nRun: npm run generate\n`,
  }
}

/** The selftest child; the env marker stops it from starting the gate again. */
export function selftestCommand() {
  return { cmd: 'node', args: ['--test', 'tools/*.test.mjs'], env: { VERIFY_SELFTEST_RUNNING: '1' } }
}

export function makeStepRunner(opts) {
  const server = join(root, 'server')
  const web = join(root, 'web')
  return async function runStep(step) {
    switch (step.name) {
      case SELFTEST: {
        const { cmd, args, env } = selftestCommand()
        return runChecked(cmd, args, root, env)
      }
      case 'go-mod-tidy':
        return runChecked('go', ['mod', 'tidy', '-diff'], server)
      case 'go-generate-drift': {
        // oapi-codegen's -o loses against the config's `output`, so point a
        // copy of the config at a temp file and leave the working tree alone.
        const tmp = mkdtempSync(join(tmpdir(), 'mirror-gen-'))
        try {
          const config = readFileSync(join(server, 'internal/api/oapi-codegen.yaml'), 'utf8')
          const out = join(tmp, 'api.gen.go')
          const cfg = join(tmp, 'oapi-codegen.yaml')
          if (!/^output:.*$/m.test(config)) throw new Error('oapi-codegen.yaml has no output line')
          writeFileSync(cfg, config.replace(/^output:.*$/m, `output: ${JSON.stringify(out)}`))
          const r = await runChecked(
            'go',
            ['tool', 'oapi-codegen', '-config', cfg, join(root, 'api/openapi.yaml')],
            server,
          )
          if (!r.ok) return r
          return driftCheck(out, join(server, 'internal/api/api.gen.go'), r.output)
        } finally {
          rmSync(tmp, { recursive: true, force: true })
        }
      }
      case 'gofmt': {
        const { code, output } = await run('gofmt', ['-l', '.'], server)
        if (code === 0 && output.trim() === '') return { ok: true, output: '$ gofmt -l .\n' }
        return { ok: false, output: `$ gofmt -l .\nNot gofmt-formatted (run gofmt -w):\n${output}` }
      }
      case 'go-vet':
        return runChecked('go', ['vet', './...'], server)
      case 'go-test':
        return runChecked('go', ['test', '-count=1', ...(opts.race ? ['-race'] : []), './...'], server)
      case 'api-types-drift': {
        const tmp = mkdtempSync(join(tmpdir(), 'mirror-types-'))
        try {
          const out = join(tmp, 'schema.gen.ts')
          const r = await runChecked(
            'npx',
            ['openapi-typescript', join(root, 'api/openapi.yaml'), '-o', out],
            web,
          )
          if (!r.ok) return r
          return driftCheck(out, join(web, 'src/shared/api/schema.gen.ts'), r.output)
        } finally {
          rmSync(tmp, { recursive: true, force: true })
        }
      }
      case 'typecheck':
        return runChecked('npx', ['tsc', '-b'], web)
      case 'lint':
        return runChecked('npm', ['run', 'lint'], web)
      case 'vitest':
        return runChecked('npx', ['vitest', 'run'], web)
      case 'build': {
        // Never into server/internal/web/dist: the backend strand compiles
        // that embedded directory at the same time.
        const tmp = mkdtempSync(join(tmpdir(), 'mirror-build-'))
        try {
          return await runChecked('npx', ['vite', 'build', '--outDir', join(tmp, 'dist'), '--emptyOutDir'], web)
        } finally {
          rmSync(tmp, { recursive: true, force: true })
        }
      }
      default:
        throw new Error(`no runner for step ${step.name}`)
    }
  }
}

export async function main(argv = process.argv.slice(2)) {
  const parsed = parseArgs(argv)
  if (parsed.error) {
    console.error(`verify: ${parsed.error}\nvalid steps: ${STEP_NAMES.join(', ')}`)
    return 2
  }
  const steps = planSteps(parsed.opts)
  const { results, exitCode } = await runGate({
    steps,
    runStep: makeStepRunner(parsed.opts),
    sequential: parsed.opts.sequential,
  })
  for (const strand of ['pre', BACKEND, FRONTEND]) {
    const log = logFor(results, strand)
    if (log) console.log(`\n===== ${strand} =====\n${log}`)
  }
  console.log(`\n${formatSummary(results)}`)
  console.log(exitCode === 0 ? '\nverify: ok' : '\nverify: FAILED')
  return exitCode
}

const direct = process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href
if (direct) process.exitCode = await main()
