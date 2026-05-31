---
name: using-tod
description: Drive OneDev (issues, pull requests, builds) through the `tod` CLI. 
  Use whenever the user asks to create, edit, query, change  state, comment on, review, 
  checkout, run CI/CD job for, get commit message requirement, or otherwise interact 
  with anything in OneDev.
---

# Using the tod CLI

This skill is the umbrella guide for driving OneDev through the `tod` CLI.
More specialized skills (`edit-build-spec`, `investigate-build-problems`,
`review-pull-request`, `generate-commit-message`, `work-on-issue`,
`submit-work`) cover specific workflows in more detail — read them when
their description matches the user's intent.

## Prerequisites

- `tod` is installed and on `PATH`.
- A tod config file (`$XDG_CONFIG_HOME/tod/config` or
  `~/.config/tod/config`) has `server-url` and `access-token` set. Run
  `tod config set` to create one.
- For commands that operate on the current project, the working directory is
  inside a git repository whose remote points at that OneDev project.

## Command shape

Commands are grouped by resource:

- `tod issue` — `list`, `get`, `get-comments`, `create`, `edit`, `change-state`,
  `link`, `add-comment`, `log-work`, `current-reference`,
  `get-query-description`, `get-valid-fields`, `get-valid-links`
- `tod pr` — `list`, `get`, `get-comments`, `get-code-comments`, `get-patch`,
  `get-file-content`, `create`, `edit`, `approve`, `request-changes`, `merge`,
  `discard`, `add-comment`, `add-code-comment`, `checkout`, `get-query-description`
- `tod code-comment` — `add-reply`, `resolve`, `unresolve` (operates on the
  `id` returned by `tod pr get-code-comments`)
- `tod build` — `list`, `get`, `get-log`, `get-file-content`,
  `get-changes-since-success`, `run` (with `--branch`, `--tag`, or `--local`),
  `get-spec-schema`, `check-spec`, `get-query-description`
- `tod config` — `init`, `show`, `path`
- `tod get-login-name`, `tod get-unix-timestamp`, `tod project`,
  `tod remote`, `tod get-valid-labels`,
  `tod get-commit-message-requirement`, `tod download`

Every command writes the raw OneDev server response to stdout. Run
`tod <command> --help` at any time to see exact flag names.

## Reference formats

Commands that take a `<ref>/<source ref>/<target ref>` argument accept:

- Issues / pull requests / builds: `#<n>`, `<project>#<n>`, or
  `<project-key>-<n>` (e.g. `#123`, `myproject#123`, `PROJ-123`).

When the user wants to act on "the current issue" without naming a
reference, run `tod issue current-reference` first. It prints `#<n>`
when the current branch matches `[<prefix>/]issue-<n>[-<suffix>]` and the
current project can be inferred, otherwise it prints nothing. Treat empty
output as "no current issue" and ask the user for an explicit reference.

## Markdown attachments in issues and PRs

Issue and pull request text often links to images or files in
descriptions and comments. When using `work-on-issue` or
`review-pull-request`, **always** download and inspect those links — do
not rely on link or alt text alone.

For each `![...](url)` or `[...](url)`, save with the URL exactly as
written in the markdown:

```bash
tod download <resource-url> <output-file>
```

Then open images with the Read tool and read other files as needed.

## Requesting user consent for state-changing calls

Ask the user to confirm before running any command that changes OneDev
state: `create`, `edit`, `change-state`, `link`, `add-comment`,
`add-code-comment`, `add-reply`, `resolve`, `unresolve`, `log-work`,
`approve`, `request-changes`, `merge`, `discard`, `run` etc. Various
list/get/show commands do not need consent.
