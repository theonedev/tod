# TOD Agent Skills

This directory ships nine agent-agnostic skills that teach an AI assistant how
to drive common OneDev workflows through the `tod` CLI. Each skill is a
self-contained folder with a `SKILL.md` file using the standard
frontmatter-plus-markdown layout supported by Claude Code, Cursor, and any
other agent that consumes `SKILL.md` files.

## Available skills

| Skill | Purpose |
|-------|---------|
| [using-tod](using-tod/SKILL.md) | Perform general OneDev queries and actions with `tod`. |
| [edit-build-spec](edit-build-spec/SKILL.md) | Author and validate `.onedev-buildspec.yml`. |
| [investigate-build-failure](investigate-build-failure/SKILL.md) | Diagnose a failed or suspicious build. |
| [review-pull-request](review-pull-request/SKILL.md) | Review a pull request and act on the findings. |
| [generate-commit-message](generate-commit-message/SKILL.md) | Compose a commit message that satisfies OneDev requirements. |
| [work-on-issue](work-on-issue/SKILL.md) | Check out and implement work for an issue. |
| [submit-issue-work](submit-issue-work/SKILL.md) | Commit and push issue work, then update or create its pull request. |
| [work-on-pull-request](work-on-pull-request/SKILL.md) | Check out a pull request and implement follow-up work. |
| [submit-pull-request-work](submit-pull-request-work/SKILL.md) | Commit and push work for an existing pull request. |

## Installing the skills

```bash
# Install tod skills for auto-detected agent(s)
npx skills add https://code.onedev.io/onedev/tod.git
```

## Prerequisites

All skills invoke the `tod` CLI, so the agent's shell must have `tod` on its
`PATH` and configured by running `tod config set`. See
[../readme.md](../readme.md) and [../cli.md](../cli.md) for setup details.
