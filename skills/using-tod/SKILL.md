---
name: using-tod
description: Drive OneDev (issues, pull requests, builds, packages, build specs) through the `tod` CLI. Use whenever the user asks to create, edit, query, transition, comment on, review, checkout, run CI for, or otherwise interact with anything in OneDev. Run `tod schema show <name>` before guessing field, state, link-name, or query-key values, since those are configured per OneDev server.
---

# Using the tod CLI

This skill is the umbrella guide for driving OneDev through the `tod` CLI.
More specialized skills (`edit-build-spec`, `investigate-build-problems`,
`review-pull-request`) cover specific workflows in more detail — read them
when their description matches the user's intent.

## Prerequisites

- `tod` is installed and on `PATH`.
- A tod config file (`$XDG_CONFIG_HOME/tod/config`, `~/.config/tod/config`,
  or the legacy `~/.todconfig`) has `server-url` and `access-token` set. Run
  `tod config init` to create one.
- For commands that operate on the current project, the working directory is
  inside a git repository whose remote points at that OneDev project.

## Command shape

Commands are grouped by resource:

- `tod issue` — `list`, `show`, `comments`, `create`, `edit`, `transition`,
  `link`, `comment`, `log-work`
- `tod pr` — `list`, `show`, `comments`, `code-comments`, `file-changes`,
  `file-content`, `create`, `edit`, `process`, `comment`, `checkout`
- `tod build` — `list`, `show`, `log`, `file-content`,
  `changes-since-success`, `run`, `run-local`, `spec`, `check-spec`
- `tod pack` — `list`
- `tod schema` — `list`, `show <name>` (see below)
- `tod config` — `init`, `show`, `path`
- `tod whoami`, `tod unix-time`, `tod project`, `tod remote`

Every command writes the raw OneDev server response to stdout. Run
`tod <command> --help` at any time to see exact flag names.

## Discover server-specific values with `tod schema`

Some values (custom fields, workflow states, link names, query keys) are
configured per OneDev server and cannot be hard-coded. Before running any of
the commands below, fetch the server's authoritative schema:

| About to run | Run this first |
|---|---|
| `tod issue create --field …` | `tod schema show create-issue` |
| `tod issue edit --field …` | `tod schema show edit-issue` |
| `tod issue transition --state … --field …` | `tod schema show change-issue-state` |
| `tod issue link --link-name …` | `tod schema show link-issues` |
| `tod issue list --query …` | `tod schema show query-issues` |
| `tod pr create --field …` | `tod schema show create-pull-request` |
| `tod pr edit --field …` | `tod schema show edit-pull-request` |
| `tod pr list --query …` | `tod schema show query-pull-requests` |
| `tod build list --query …` | `tod schema show query-builds` |
| `tod pack list --query …` | `tod schema show query-packs` |

Use `jq` to keep the context footprint small — you usually only need the key
names or an enum list, not the full schema:

```bash
# Valid flag keys for one command
tod schema show create-issue | jq -r '.properties | keys[]'

# Required keys + short descriptions
tod schema show create-issue | jq '{required, props: (.properties | to_entries | map({key, desc: .value.description}))}'

# Allowed values for an enum field
tod schema show change-issue-state | jq '.properties.state'
```

If you only need to know *what* schemas exist, `tod schema list` is tiny
(~200 bytes).

When to **skip** schema lookup: if the user only asks for well-known static
flags (`--title`, `--description`, `--branch`, `--tag`, `--working-dir`,
`--path`, `--content`, `--operation`, `--hours`, `-p key=value`, etc.), they
are already visible in `tod <command> --help` and you don't need
`tod schema`.

## Reference formats

Commands that take a `<ref>` argument accept:

- Issues / pull requests / builds: `#<n>`, `<project>#<n>`, or
  `<project-key>-<n>` (e.g. `#123`, `myproject#123`, `PROJ-123`).

## `--field key=value` encoding

Commands that accept arbitrary extra fields (`issue create`/`edit`/
`transition`, `pr create`/`edit`) parse each `--field` value as JSON first
and fall back to a string. Examples:

```bash
--field confidential=true                 # boolean true
--field iterations='["2.0","3.0"]'        # array of strings
--field points=5                          # number 5
--field title='"Fix crash"'               # explicit JSON string
--field title="Fix crash"                 # plain string (quotes absorbed by shell)
```

## Requesting user consent for state-changing calls

Ask the user to confirm before running any command that changes OneDev
state: `create`, `edit`, `transition`, `link`, `comment`, `log-work`,
`process`, `run`, `run-local`, `checkout`. Read-only commands (`list`,
`show`, `comments`, `code-comments`, `file-changes`, `file-content`, `log`,
`changes-since-success`, `schema`, `whoami`, `unix-time`, `project`,
`remote`) do not need consent.
