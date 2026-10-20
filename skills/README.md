# TOD Agent Skills

This directory ships eight agent-agnostic skills that teach an AI assistant how
to drive common OneDev workflows through the `tod` CLI. Each skill is a
self-contained folder with a `SKILL.md` file using the standard
frontmatter-plus-markdown layout supported by Claude Code, Cursor, and any
other agent that consumes `SKILL.md` files.

## Available skills

| Skill | Purpose |
|-------|---------|
| [using-tod](using-tod/SKILL.md) | Perform general OneDev queries and actions with `tod`. |
| [edit-build-spec](edit-build-spec/SKILL.md) | Author and validate `.onedev-buildspec.yml`. |
| [fix-failed-build](fix-failed-build/SKILL.md) | Fix a failed build from a prompt-provided build reference. |
| [generate-commit-message](generate-commit-message/SKILL.md) | Compose a commit message that satisfies OneDev requirements. |
| [work-on-issue](work-on-issue/SKILL.md) | Check out and implement work for an issue. |
| [submit-issue-work](submit-issue-work/SKILL.md) | Submit issue code changes and/or saved comments; update or create a PR when submitting code. |
| [work-on-pull-request](work-on-pull-request/SKILL.md) | Review a pull request or implement follow-up work. |
| [submit-pull-request-work](submit-pull-request-work/SKILL.md) | Submit code changes and/or saved discussion, review, and merge actions for an existing PR. |

## Installing the skills

```bash
# Install tod skills for auto-detected agent(s)
npx skills add https://code.onedev.io/onedev/tod.git
```

## Prerequisites

All skills invoke the `tod` CLI, so the agent's shell must have `tod` on its
`PATH` and configured by running `tod config set`. See
[../readme.md](../readme.md) and [../cli.md](../cli.md) for setup details.

All skills except `using-tod` require the working directory (either the current
directory or one specified by the user) to be inside a Git repository belonging
to the relevant OneDev project. This also applies to review-only work and
submitting saved comments or PR actions. Each skill states any additional
project, branch, or project-root requirements.

`using-tod` can perform API-only work without a checkout using qualified
references and explicit project flags. Its local repository commands still
require a checkout. See the
[CLI context guide](../cli.md#project-and-repository-context) for command
requirements and options for working without a checkout.
