# TOD Agent Skills

TOD ships four agent-agnostic skills under [`skills/`](skills/). Each is a
self-contained folder with a `SKILL.md` file using the standard
frontmatter-plus-markdown layout supported by Claude Code, Cursor, and any
other agent that consumes `SKILL.md` files. Each skill drives a common
OneDev workflow through the `tod` CLI (see [cli.md](cli.md)).

## Available skills

| Skill | Purpose |
|-------|---------|
| [`using-tod`](skills/using-tod/SKILL.md) | Umbrella guide that covers any OneDev interaction and points the agent at `tod schema show <name>` for discovering server-specific fields, states, link names, and query keys. |
| [`edit-build-spec`](skills/edit-build-spec/SKILL.md) | Create or update `.onedev-buildspec.yml` using `tod build spec` and `tod build check-spec`. |
| [`investigate-build-problems`](skills/investigate-build-problems/SKILL.md) | Debug a failing build via `tod build show`, `tod build log`, `tod build file-content`, and `tod build changes-since-success`. |
| [`review-pull-request`](skills/review-pull-request/SKILL.md) | Review a pull request via `tod pr show`, `tod pr file-changes`, `tod pr file-content`, and `tod pr process` or `tod pr comment`. |

## Installing

These folders can be dropped into whichever skills directory your agent reads
from. Some common choices:

- **Claude Code (personal)**: `~/.claude/skills/`
- **Claude Code (project)**: `<repo>/.claude/skills/`
- **Cursor (personal)**: `~/.cursor/skills/`
- **Cursor (project)**: `<repo>/.cursor/skills/`

Copy, symlink, or reference these folders however your agent prefers. For
example:

```bash
ln -s "$PWD/skills/review-pull-request" ~/.claude/skills/review-pull-request
```

## Prerequisites

All skills invoke the `tod` CLI, so the agent's shell must have `tod` on its
`PATH` and a configured tod config file (`$XDG_CONFIG_HOME/tod/config`,
`~/.config/tod/config`, or the legacy `~/.todconfig`) with `server-url` and
`access-token`. Run `tod config init` to create one. See the main
[readme.md](readme.md) and [cli.md](cli.md) for setup details.

## Writing new skills

Follow the same pattern: a lowercase-hyphen folder name, a `SKILL.md` with
`name` and `description` frontmatter keys, and a concise body that instructs
the agent which `tod` commands to run. Keep each `SKILL.md` under ~200 lines
and push long reference material into sibling files.
