// Regenerates CHANGELOG.md from the Conventional Commit history with git-cliff.
// CHANGELOG.md is generated - never edit it by hand (CLAUDE.md -> Workflow).
//
//   node tools/changelog.mjs                                   regenerate
//   node tools/changelog.mjs --tag v0.2.0 --message "text"     annotated tag on HEAD, then regenerate
//   node tools/changelog.mjs --with-commit "feat(x): subject"  include a commit not yet made
//
// Needs git-cliff on PATH, or Node/npx (fetches git-cliff@2).
import { spawnSync } from 'node:child_process'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const TAG_RE = /^v\d+\.\d+\.\d+$/

/** Parses CLI args; never exits. Returns `{ opts }` or `{ error }`. */
export function parseArgs(argv) {
  const opts = { tag: null, message: null, withCommit: null }
  const flags = { '--tag': 'tag', '--message': 'message', '--with-commit': 'withCommit' }
  for (let i = 0; i < argv.length; i++) {
    const key = flags[argv[i]]
    if (!key) return { error: `unknown argument: ${argv[i]}` }
    const value = argv[++i]
    if (value === undefined || value === '') return { error: `${argv[i - 1]} needs a value` }
    opts[key] = value
  }
  if (opts.tag && !opts.message) {
    return { error: '--message is required with --tag (an annotated tag needs a description)' }
  }
  if (opts.message && !opts.tag) return { error: '--message is only valid together with --tag' }
  if (opts.tag && !TAG_RE.test(opts.tag)) {
    return { error: `tag must look like v0.2.0, got: ${opts.tag}` }
  }
  return { opts }
}

export function cliffArgs(opts) {
  const args = ['-c', 'cliff.toml', '-o', 'CHANGELOG.md']
  if (opts.withCommit) args.push('--with-commit', opts.withCommit)
  return args
}

function sh(cmd, args) {
  // npx is a .cmd shim on Windows; arguments here are fixed or validated.
  const r = spawnSync(cmd, args, {
    cwd: root,
    stdio: 'inherit',
    shell: process.platform === 'win32' && cmd === 'npx',
  })
  return r.status ?? 1
}

function hasCliff() {
  return spawnSync('git-cliff', ['--version'], { stdio: 'ignore' }).status === 0
}

export function main(argv = process.argv.slice(2)) {
  const parsed = parseArgs(argv)
  if (parsed.error) {
    console.error(`changelog: ${parsed.error}`)
    return 2
  }
  const { opts } = parsed
  if (opts.tag) {
    const code = sh('git', ['tag', '-a', opts.tag, '-m', opts.message])
    if (code !== 0) return code
    console.log(`Tag ${opts.tag} set on HEAD.`)
  }
  const args = cliffArgs(opts)
  const code = hasCliff() ? sh('git-cliff', args) : sh('npx', ['--yes', 'git-cliff@2', ...args])
  if (code === 0) console.log('CHANGELOG.md regenerated.')
  return code
}

const direct = process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href
if (direct) process.exitCode = main()
