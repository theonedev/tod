# TOD Agent Skills

This directory ships nine agent-agnostic skills that teach an AI assistant how
to drive common OneDev workflows through the `tod` CLI. Each skill is a
self-contained folder with a `SKILL.md` file using the standard
frontmatter-plus-markdown layout supported by Claude Code, Cursor, and any
other agent that consumes `SKILL.md` files.

## Available skills

| Skill | Purpose |
|-------|---------|
| [using-tod](using-tod/SKILL.md) | Umbrella guide for any OneDev interaction via `tod`. |
| [edit-build-spec](edit-build-spec/SKILL.md) | Create or update `.onedev-buildspec.yml` using `tod build get-spec-schema` and `tod build check-spec`. |
| [investigate-build-failure](investigate-build-failure/SKILL.md) | Debug a failing build via `tod build get`, `tod build get-log`, and related commands. |
| [review-pull-request](review-pull-request/SKILL.md) | Review a pull request via `tod pr get`, `tod pr get-patch`, and friends. |
| [generate-commit-message](generate-commit-message/SKILL.md) | Compose a commit message that respects `tod get-commit-message-requirement` (and `tod pr get-commit-message-requirement` when submitting PR work), and adds a meaningful body or footer. |
| [work-on-issue](work-on-issue/SKILL.md) | Work on a OneDev issue: `tod issue create-branch`, check out the issue branch, read context, and implement. |
| [submit-issue-work](submit-issue-work/SKILL.md) | Submit completed issue work: commit pending changes (via `generate-commit-message` with branch and PR requirements), push the issue branch, attach to an existing open PR for the issue when one exists, otherwise open a pull request via `tod pr create`. |
| [work-on-pull-request](work-on-pull-request/SKILL.md) | Address follow-up on a pull request: verify the source project, check out with `tod pr checkout`, fix code locally, post comment-only replies immediately, and draft push-dependent replies for submit. |
| [submit-pull-request-work](submit-pull-request-work/SKILL.md) | Submit completed pull request work: commit pending changes (via `generate-commit-message` with branch and PR requirements), push the PR source branch, then post deferred replies/resolves so notifications match visible commits. |

## Installing the skills

```bash
# Install tod skills for auto-detected agent(s)
npx skills add https://code.onedev.io/onedev/tod.git
```

## Prerequisites

All skills invoke the `tod` CLI, so the agent's shell must have `tod` on its
`PATH` and configured by running `tod config set`. See
[../readme.md](../readme.md) and [../cli.md](../cli.md) for setup details.

