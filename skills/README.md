# TOD Agent Skills

This directory ships five agent-agnostic skills that teach an AI assistant how
to drive common OneDev workflows through the `tod` CLI. Each skill is a
self-contained folder with a `SKILL.md` file using the standard
frontmatter-plus-markdown layout supported by Claude Code, Cursor, and any
other agent that consumes `SKILL.md` files.

## Available skills

| Skill | Purpose |
|-------|---------|
| [`using-tod`](using-tod/SKILL.md) | Umbrella guide for any OneDev interaction via `tod`. |
| [`edit-build-spec`](edit-build-spec/SKILL.md) | Create or update `.onedev-buildspec.yml` using `tod build get-spec-schema` and `tod build check-spec`. |
| [`investigate-build-problems`](investigate-build-problems/SKILL.md) | Debug a failing build via `tod build get`, `tod build get-log`, and related commands. |
| [`review-pull-request`](review-pull-request/SKILL.md) | Review a pull request via `tod pr get`, `tod pr get-patch`, and friends. |
| [`generate-commit-message`](generate-commit-message/SKILL.md) | Compose a commit message that respects `tod get-commit-message-requirement`, derives the subject from the active issue when set, and adds a meaningful body or footer. |
| [`work-on-issue`](work-on-issue/SKILL.md) | Start work on a OneDev issue: create the issue branch on the server with `tod issue create-branch`, switch the local checkout onto it with the right remote tracking, then read the issue's title, description, and comments via `tod issue get` / `tod issue get-comments`. |
| [`submit-work`](submit-work/SKILL.md) | Submit completed work on a OneDev issue: confirm the issue branch on the server, verify the local checkout is on it, commit any pending changes (via `generate-commit-message`), push to the matching remote branch, and open a pull request via `tod pr create` with a title and description summarizing the issue branch's commits. |

## Installing the skills

These folders can be dropped into whichever skills directory your agent reads
from. Some common choices:

- **Claude Code (personal)**: `~/.claude/skills/`
- **Claude Code (project)**: `<repo>/.claude/skills/`
- **Cursor (personal)**: `~/.cursor/skills/`
- **Cursor (project)**: `<repo>/.cursor/skills/`

You can either copy the skill folders in, symlink them, or (if your agent
lets you) point it at this directory directly. For example:

```bash
ln -s "$PWD/skills/review-pull-request" ~/.claude/skills/review-pull-request
```

## Prerequisites

All skills invoke the `tod` CLI, so the agent's shell must have `tod` on its
`PATH` and a configured tod config file (`$XDG_CONFIG_HOME/tod/config`,
`~/.config/tod/config`, or the legacy `~/.todconfig`) with `server-url` and
`access-token`. Run `tod config set` to create one. See
[../readme.md](../readme.md) and [../cli.md](../cli.md) for setup details.

## Writing new skills

Follow the same pattern: a lowercase-hyphen folder name, a `SKILL.md` with
`name` and `description` frontmatter keys, and a concise body that instructs
the agent which `tod` commands to run. Keep each `SKILL.md` under ~200 lines
and push long reference material into sibling files.
