# TOD CLI reference

Every `tod` subcommand writes the raw OneDev server response to stdout (same
payload the old MCP server returned in `ToolContent.Text`) and logs progress
to stderr. Commands that need to talk to OneDev require a config file with
`server-url` and `access-token` set — `$XDG_CONFIG_HOME/tod/config`,
`~/.config/tod/config`, or the legacy `~/.todconfig` (run `tod config init` to
create one). Commands that interact with the current project also expect the
working directory to be inside a git repository whose remote points at that
project (override with `--working-dir`).

Top-level groups:

- [`tod issue`](#tod-issue)
- [`tod pr`](#tod-pr)
- [`tod build`](#tod-build)
- [`tod pack`](#tod-pack)
- [`tod schema`](#tod-schema)
- [`tod config`](#tod-config)
- [misc: `whoami`, `unix-time`, `project`, `remote`](#miscellaneous)

Reference formats accepted by `<ref>` arguments:

- Issues: `#<n>`, `<project>#<n>`, or `<project-key>-<n>`
- Pull requests: same syntax as issues
- Builds: same syntax as issues

## `tod issue`

| Command | Description |
|---------|-------------|
| `tod issue list` | Query issues. Flags: `--project`, `--query`, `--offset`, `--count`. |
| `tod issue show <ref>` | Full detail for one issue. |
| `tod issue comments <ref>` | List comments on an issue. |
| `tod issue create` | Create a new issue. Flags: `--title` (required), `--description`, `--project`, `--field key=value` (repeatable). |
| `tod issue edit <ref>` | Edit an existing issue. Flags: `--title`, `--description`, `--field key=value`. |
| `tod issue transition <ref>` | Change state. Flags: `--state` (required), `--comment`, `--field key=value`. |
| `tod issue link <source> <target>` | Add `<target>` as `--link-name` of `<source>`. |
| `tod issue comment <ref>` | Add a comment. Flag: `--content` (required). |
| `tod issue log-work <ref>` | Record time spent. Flags: `--hours` (required), `--comment`. |

## `tod pr`

| Command | Description |
|---------|-------------|
| `tod pr list` | Query pull requests. Flags: `--project`, `--query`, `--offset`, `--count`. |
| `tod pr show <ref>` | Full detail for one pull request. |
| `tod pr comments <ref>` | List general comments on a pull request. |
| `tod pr code-comments <ref>` | List code comments on a pull request. |
| `tod pr file-changes <ref>` | Patch. Flag: `--for-code-review`. |
| `tod pr file-content <ref>` | File content at a specific revision. Flags: `--path` (required), `--old-revision`. |
| `tod pr create` | Create a pull request. Flags: `--source-branch` (required), `--target-branch`, `--title`, `--description`, `--source-project`, `--target-project`, `--assignee`, `--reviewer`, `--merge-strategy`, `--field key=value`. |
| `tod pr edit <ref>` | Edit a pull request. Flags: `--title`, `--description`, `--assignee`, `--add-reviewer`, `--remove-reviewer`, `--merge-strategy`, `--field key=value`. |
| `tod pr process <ref>` | Run an operation. Flags: `--operation` (required: `approve`, `requestChanges`, `merge`, `discard`, `reopen`, `deleteSourceBranch`, `restoreSourceBranch`), `--comment`. |
| `tod pr comment <ref>` | Add a comment. Flag: `--content` (required). |
| `tod pr checkout <ref>` | Check out the pull request into the working directory. |

## `tod build`

| Command | Description |
|---------|-------------|
| `tod build list` | Query builds. Flags: `--project`, `--query`, `--offset`, `--count`. |
| `tod build show <ref>` | Full detail for one build. |
| `tod build log <ref>` | Build log. |
| `tod build file-content <ref>` | File content at the build's commit. Flag: `--path` (required). |
| `tod build changes-since-success <ref>` | Patch between the build's commit and the previous successful similar build. |
| `tod build run <job-name>` | Run a job against a branch or tag. Flags: `--branch` or `--tag`, `-p key=value`. Streams the log. |
| `tod build run-local <job-name>` | Run a job against local changes. Flag: `-p key=value`. Streams the log. |
| `tod build spec` | Print the build spec YAML definition. |
| `tod build check-spec` | Validate (and upgrade) `.onedev-buildspec.yml` in the working directory. |

## `tod pack`

| Command | Description |
|---------|-------------|
| `tod pack list` | Query packages. Flags: `--project`, `--query`, `--offset`, `--count`. |

## `tod schema`

Dynamic field definitions (custom fields, allowed states, valid link names,
query keys, merge strategies, etc.) are configured per-server in OneDev.
`tod schema` exposes the server's authoritative list so agents and scripts
can discover what values are accepted without hardcoding.

| Command | Description |
|---------|-------------|
| `tod schema list` | Print the schema names this server exposes (one per line). |
| `tod schema show <name>` | Print the JSON schema for one tool. Accepts camelCase (`createIssue`) or kebab-case (`create-issue`). |

Typical names (exact set comes from the server): `create-issue`,
`edit-issue`, `change-issue-state`, `link-issues`, `query-issues`,
`create-pull-request`, `edit-pull-request`, `query-pull-requests`,
`query-builds`, `query-packs`.

Pipe through `jq` to extract only the parts you need:

```bash
# Just the flag keys (tiny, ~100 bytes)
tod schema show create-issue | jq -r '.properties | keys[]'

# Required keys + short descriptions
tod schema show create-issue | jq '{required, props: (.properties | to_entries | map({key, desc: .value.description}))}'

# Allowed values for the state flag of `tod issue transition`
tod schema show change-issue-state | jq '.properties.state'
```

Affected commands (`tod issue create`/`edit`/`transition`/`link`,
`tod pr create`/`edit`, and all `--query` flags) reference the appropriate
`tod schema show <name>` invocation in their `--help` output, so agents can
discover the right schema on demand instead of guessing.

## `tod config`

Create or inspect the tod config file (`server-url` and `access-token`).
These commands run even when the config file is missing or invalid.

| Command | Description |
|---------|-------------|
| `tod config init` | Create or update the config file. Flags: `--server-url`, `--access-token`, `--path`, `--non-interactive`. Missing values are prompted for unless `--non-interactive` is set; existing values are preserved. The file is written with mode `0600` to `$XDG_CONFIG_HOME/tod/config`, `~/.config/tod/config`, `~/.todconfig` (whichever already exists), or `~/.config/tod/config` if none. |
| `tod config show` | Print the active configuration. The access token is redacted unless `--show-token` is given. |
| `tod config path` | Print the path of the config file (the first match in the search order, or the legacy `~/.todconfig` fallback if no file exists yet). |

## Miscellaneous

| Command | Description |
|---------|-------------|
| `tod whoami [--user <name>]` | Print the login name of the current user (or `--user`). |
| `tod unix-time <description>` | Convert a natural-language description (e.g. `today`, `next month`, `2025-01-01 12:00:00`) to a Unix timestamp in milliseconds. |
| `tod project` | Print the OneDev project inferred from the working directory. |
| `tod remote` | Print the git remote that points at the inferred OneDev project. |

## `--field key=value` encoding

Commands that accept arbitrary extra fields (`issue create`/`edit`/`transition`,
`pr create`/`edit`) parse each `--field` value as JSON first, falling back to a
string if the value is not valid JSON. Examples:

```bash
--field confidential=true                 # boolean true
--field iterations='["2.0","3.0"]'        # array of strings
--field points=5                          # number 5
--field title='"Fix crash on startup"'    # explicit JSON string
--field title="Fix crash on startup"      # plain string (quotes absorbed by shell)
```

## Deprecated aliases

The original top-level commands are kept as hidden aliases so existing
scripts continue to work, but new usage should prefer the grouped names:

| Old | New |
|-----|-----|
| `tod run <job>` | `tod build run <job>` |
| `tod run-local <job>` | `tod build run-local <job>` |
| `tod checkout <ref>` | `tod pr checkout <ref>` |
