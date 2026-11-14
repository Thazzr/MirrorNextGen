# Agent Instructions

**`CLAUDE.md` is the single source of truth** for the rules and conventions of this repository: workflow, architecture boundaries, Go/frontend/testing rules, the verification gate and the per-phase commit, tag and changelog process. Read it first.

This file exists only so agents that look for `AGENTS.md` by convention find the pointer. It is intentionally **not** a second copy of the rules, because a duplicated ruleset drifts out of sync.

- Operative rules & workflow → [`CLAUDE.md`](CLAUDE.md)
- Agent roles → [`.claude/agents/implementer.md`](.claude/agents/implementer.md), [`.claude/agents/mutation-checker.md`](.claude/agents/mutation-checker.md)
- Generated changelog (do not hand-edit) → [`CHANGELOG.md`](CHANGELOG.md), config [`cliff.toml`](cliff.toml)
