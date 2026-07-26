---
name: using-tod
description: Operate OneDev issues, pull requests, builds, and projects with `tod`. Use for general OneDev queries and actions not covered by a more specific skill.
---

# Using the tod CLI

Use the command and consent rules below for general OneDev interactions.

## Prerequisites

- `tod` is installed and configured with `server-url` and `access-token`.
- Project-scoped commands run inside the corresponding OneDev repository.

## Command shape

Commands are grouped by resource:

- `tod issue` -- `list`, `get`, `get-comments`, `create`, `edit`, `change-state`,
  `link`, `add-comment`, `log-work`, `create-branch`, `checkout`,
  `current-reference`,
  `get-query-description`, `get-valid-fields`, `get-valid-links`
- `tod pr` -- `list`, `get`, `get-comments`, `get-code-comments`, `get-builds`, `get-patch`,
  `create`, `get-title-and-description-requirement`,
  `get-commit-message-requirement`, `edit`, `approve`, `request-changes`, `merge`,
  `discard`, `add-comment`, `add-code-comment`, `checkout`, `get-query-description`
- `tod code-comment` -- `add-reply`, `resolve`, `unresolve` (operates on the
  `id` returned by `tod pr get-code-comments`)
- `tod build` -- `list`, `get`, `get-log`, `get-code-problems`,
  `get-changes-since-success`, `run` (with `--branch`, `--tag`, or `--local`),
  `get-spec-schema`, `check-spec`, `get-query-description`
- `tod config` -- `set`, `get`, `path`
- `tod project` -- `list`, `current`, `get`, `get-query-description`
- `tod get-login-name`, `tod get-unix-timestamp`, `tod remote`,
  `tod get-valid-labels`,
  `tod get-commit-message-requirement`, `tod download`

Commands write the raw OneDev server response to stdout. Run
`tod <command> --help` for exact flags.

`tod build get-code-problems <build-reference> <report-name> <severity-level>`
returns found code problems for a build with or higher than the specified
severity. `<severity-level>` must be one of `CRITICAL`, `HIGH`, `MEDIUM`, or
`LOW`.

## CLI reference formats

The following forms are for `tod` command arguments. The command selects the
entity type; these forms are not the syntax to copy into authored Markdown.

Commands that take a `<ref>/<source ref>/<target ref>` argument accept:

- Issues / pull requests / builds: `<n>`, `#<n>`, `<project>#<n>`, or
  `<project-key>-<n>` (e.g. `123`, `#123`, `myproject#123`, `PROJ-123`).

When the user wants to act on "the current issue" without naming a
reference, run `tod issue current-reference` first. It prints `<n>` when the
current branch matches `[<prefix>/]issue-<n>[-<suffix>]`; otherwise it prints
an error and exits non-zero. On failure, ask the user for an explicit
reference.

## Markdown entity references

Write `<type> <reference>`, where type is `issue`, `pr` (or `pull request`),
or `build`. Use `#123` in the same project, `path/to/project#123` across
projects, or `PROJ-123` when the project has a key. For example: `issue #123`,
`pr path/to/project#123`, and `build PROJ-123`.

A bare `#123` always means an issue; never use it for a pull request or build.
These Markdown forms differ from the CLI forms above.

## Markdown attachments in issues and PRs

Issue and pull request text often links to images or files in
descriptions and comments. When using `work-on-issue` or
`work-on-pull-request`, **always** download and inspect those links -- do not
rely on link or alt text alone.

For each `![...](url)` or `[...](url)`, save with the URL exactly as
written in the markdown:

```bash
tod download <resource-url> <output-file>
```

Then inspect images and read other files as needed.

## Requesting user consent for state-changing calls

Ask the user to confirm before running any command that changes OneDev
state: `create`, `edit`, `change-state`, `link`, `add-comment`,
`add-code-comment`, `add-reply`, `resolve`, `unresolve`, `log-work`,
`create-branch`, `checkout`, `approve`, `request-changes`, `merge`, `discard`, `run` etc. Various
list/get commands do not need consent.
