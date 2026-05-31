# TOD CLI reference

Every `tod` subcommand writes the raw OneDev server response to stdout (same
payload the old MCP server returned in `ToolContent.Text`) and logs progress
to stderr. Commands that need to talk to OneDev require a config file with
`server-url` and `access-token` set — `$XDG_CONFIG_HOME/tod/config` or
`~/.config/tod/config` (run `tod config set` to create one). Commands that
interact with the current project also expect the working directory to be
inside a git repository whose remote points at that project (override with
`--working-dir`).

Top-level groups:

- [`tod issue`](#tod-issue)
- [`tod pr`](#tod-pr)
- [`tod code-comment`](#tod-code-comment)
- [`tod build`](#tod-build)
- [`tod project`](#tod-project)
- [`tod config`](#tod-config)
- [misc: `get-login-name`, `get-unix-timestamp`, `remote`, `get-valid-labels`, `get-commit-message-requirement`, `download`](#miscellaneous)

Reference formats accepted by `<ref>`/`<target ref>`/`<source ref>` arguments:

- Issues: `#<n>`, `<project>#<n>`, or `<project-key>-<n>`
- Pull requests: same syntax as issues
- Builds: same syntax as issues

## `tod issue`

| Command | Description |
|---------|-------------|
| `tod issue list` | Query issues. Flags: `--project`, `--query`, `--offset`, `--count`. |
| `tod issue get <ref>` | Detail information of a issue except comments. |
| `tod issue get-comments <ref>` | List comments on an issue. |
| `tod issue create <title>` | Create a new issue. Flags: `--description`, `--project`, `--field key=value` (repeatable), `--iteration`, `--own-estimated-time`, `--confidential`. |
| `tod issue edit <ref>` | Edit an existing issue. Flags: `--title`, `--description`, `--field key=value` (repeatable), `--iteration`, `--own-estimated-time`, `--confidential`. |
| `tod issue change-state <ref> <state>` | Change state. Flags: `--comment`, `--field key=value` (repeatable). |
| `tod issue link <source ref> <link-name> <target ref>` | Add `<target ref>` as `<link-name>` of `<source ref>` (run `tod issue get-valid-links` for valid link names). |
| `tod issue add-comment <ref> <content>` | Add a comment. |
| `tod issue log-work <ref> <hours>` | Record time spent. Flag: `--comment`. |
| `tod issue create-branch <ref>` | Create a branch on the server for the specified issue. |
| `tod issue current-reference` | Print the issue reference (`#<n>`) inferred from the current branch (matched against `[<prefix>/]issue-<n>[-<suffix>]`). Prints nothing when the current project cannot be inferred or the branch does not match. |
| `tod issue get-query-description` | Print the OneDev issue query DSL description (syntax reference for `--query` of `issue list`). |
| `tod issue get-valid-fields` | Print valid issue fields and their allowed values (use before passing `--field` to `create`/`edit`/`change-state`). |
| `tod issue get-valid-links` | Print valid issue link names (use as the `<link-name>` argument to `issue link`). |

## `tod pr`

| Command | Description |
|---------|-------------|
| `tod pr list` | Query pull requests. Flags: `--project`, `--query`, `--offset`, `--count`. |
| `tod pr get <ref>` | Detail information of a pull request except comments and code comments. |
| `tod pr get-comments <ref>` | List general comments on a pull request. |
| `tod pr get-code-comments <ref>` | List code comments on a pull request. |
| `tod pr get-patch <ref>` | Patch. Flag: `--for-code-review`. |
| `tod pr get-file-content <ref> <path>` | File content at a specific revision. Flag: `--old-revision`. |
| `tod pr create <title>` | Create a pull request. Flags: `--source-branch` (defaults to current git branch), `--target-branch`, `--description`, `--source-project`, `--target-project`, `--assignee`, `--reviewer`, `--label`, `--merge-strategy`. |
| `tod pr edit <ref>` | Edit a pull request. Flags: `--title`, `--description`, `--assignee`, `--add-reviewer`, `--remove-reviewer`, `--label`, `--merge-strategy`, `--auto-merge`, `--auto-merge-commit-message`. |
| `tod pr approve <ref>` | Approve a pull request as a pending reviewer. Flag: `--comment`. |
| `tod pr request-changes <ref>` | Request changes on a pull request as a pending reviewer. Flag: `--comment`. |
| `tod pr merge <ref>` | Merge a pull request. Flag: `--commit-message`. |
| `tod pr discard <ref>` | Discard (close without merging) a pull request. Flag: `--comment`. |
| `tod pr add-comment <ref> <content>` | Add a comment. |
| `tod pr add-code-comment <ref> <content>` | Add a code comment to a line range visible on the right side of the PR patch. Flags: `--file` (required), `--from-line` (required, 1-based), `--to-line` (defaults to `--from-line`). |
| `tod pr checkout <ref>` | Check out the pull request into the working directory. |
| `tod pr get-query-description` | Print the OneDev pull request query DSL description (syntax reference for `--query` of `pr list`). |

## `tod code-comment`

Code comment IDs are returned by `tod pr get-code-comments <pr-reference>` (the
`id` field on each entry).

| Command | Description |
|---------|-------------|
| `tod code-comment add-reply <comment-id> <content>` | Add a reply to a code comment. |
| `tod code-comment resolve <comment-id>` | Mark a code comment as resolved. Flag: `--note`. |
| `tod code-comment unresolve <comment-id>` | Mark a code comment as unresolved. Flag: `--note`. |

## `tod build`

| Command | Description |
|---------|-------------|
| `tod build list` | Query builds. Flags: `--project`, `--query`, `--offset`, `--count`. |
| `tod build get <ref>` | Detail information of a build. |
| `tod build get-log <ref>` | Build log. |
| `tod build get-file-content <ref> <path>` | File content at the build's commit. |
| `tod build get-changes-since-success <ref>` | Patch between the build's commit and the previous successful similar build. |
| `tod build run <job-name>` | Run a job against a branch, tag, or local changes. Flags: exactly one of `--branch`, `--tag`, or `--local`; `-p key=value`. Streams the log. |
| `tod build get-spec-schema` | Print the build spec YAML definition. |
| `tod build check-spec` | Validate (and upgrade) `.onedev-buildspec.yml` in the working directory. |
| `tod build get-query-description` | Print the OneDev build query DSL description (syntax reference for `--query` of `build list`). |

## `tod project`

| Command | Description |
|---------|-------------|
| `tod project current` | Print the OneDev project inferred from the working directory. |
| `tod project get <project-path>` | Print info of the specified project. |

## `tod config`

Create or inspect the tod config file (`server-url` and `access-token`).
These commands run even when the config file is missing or invalid.

| Command | Description |
|---------|-------------|
| `tod config set` | Create or update the config file interactively. You are prompted for each property in turn: the current server URL is shown in `[brackets]` as a default (press Enter to keep, or type to replace), and the access-token prompt is always blank (press Enter to keep the existing token, or type a new one). For non-interactive single-property updates, use `tod config set <property name> <property value>`. The file is written with mode `0600` to whichever of `$XDG_CONFIG_HOME/tod/config` or `~/.config/tod/config` already exists, otherwise to `~/.config/tod/config` (or `$XDG_CONFIG_HOME/tod/config` when set). |
| `tod config set <property name> <property value>` | Set one config property. Property name must be `server-url` or `access-token`. |
| `tod config get [property name]` | Print the active configuration or one config property. The access token is always redacted. |
| `tod config path` | Print the path of the config file (the first match in the search order, or the default destination if no file exists yet). |

## Miscellaneous

| Command | Description |
|---------|-------------|
| `tod get-login-name [--user <name>]` | Print the login name of the current user (or `--user`). |
| `tod get-unix-timestamp <description>` | Convert a natural-language description (e.g. `today`, `next month`, `2025-01-01 12:00:00`) to a Unix timestamp in milliseconds. |
| `tod remote` | Print the git remote that points at the inferred OneDev project. |
| `tod get-valid-labels` | Print valid label names for this OneDev server (use with `--label` in `tod pr create`). |
| `tod get-commit-message-requirement` | Print the commit message requirement for non-merge commits in the current project (inferred from the working directory). Prints nothing when the project has no requirement configured. |
| `tod download <resource-url> <output-file>` | Download a resource (image, file, etc.) referenced in markdown. The resource URL is the original URL from the markdown without modification. Relative URLs are resolved against `server-url`; authentication uses `access-token`. |
