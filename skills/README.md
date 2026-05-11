# TOD Agent Skills

This directory ships seven agent-agnostic skills that teach an AI assistant how
to drive common OneDev workflows through the `tod` CLI. Each skill is a
self-contained folder with a `SKILL.md` file using the standard
frontmatter-plus-markdown layout supported by Claude Code, Cursor, and any
other agent that consumes `SKILL.md` files.

## Available skills

| Skill | Purpose |
|-------|---------|
| [`using-tod`](using-tod/SKILL.md) | Umbrella guide for any OneDev interaction via `tod`. |
| [`edit-build-spec`](edit-build-spec/SKILL.md) | Create or update `.onedev-buildspec.yml` using `tod build get-spec-schema` and `tod build check-spec`. |
| [`investigate-build-failure`](investigate-build-failure/SKILL.md) | Debug a failing build via `tod build get`, `tod build get-log`, and related commands. |
| [`review-pull-request`](review-pull-request/SKILL.md) | Review a pull request via `tod pr get`, `tod pr get-patch`, and friends. |
| [`generate-commit-message`](generate-commit-message/SKILL.md) | Compose a commit message that respects `tod get-commit-message-requirement`, derives the subject from the active issue when set, and adds a meaningful body or footer. |
| [`work-on-issue`](work-on-issue/SKILL.md) | Start work on a OneDev issue: create the issue branch on the server with `tod issue create-branch`, switch the local checkout onto it with the right remote tracking, then read the issue's title, description, and comments via `tod issue get` / `tod issue get-comments`. |
| [`submit-work`](submit-work/SKILL.md) | Submit completed work on a OneDev issue: confirm the issue branch on the server, verify the local checkout is on it, commit any pending changes (via `generate-commit-message`), push to the matching remote branch, and open a pull request via `tod pr create` with a title and description summarizing the issue branch's commits. |

## Installing the skills

```bash
# Install all tod skills for the auto-detected agent(s) in the current project
npx skills add https://code.onedev.io/onedev/tod/skills
```

## Prerequisites

All skills invoke the `tod` CLI, so the agent's shell must have `tod` on its
`PATH` and configured by running `tod config set`. See
[../readme.md](../readme.md) and [../cli.md](../cli.md) for setup details.

