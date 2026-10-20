# TOD CLI reference

API commands generally write the raw OneDev server response to stdout.
Commands that operate on local files or run builds also print status and
progress. Commands that need to talk to OneDev require a config file with
`server-url` and `access-token` set — `$XDG_CONFIG_HOME/tod/config` or
`~/.config/tod/config` (run `tod config set` to create one). Set the optional
`trust-certs-file` property to trust additional PEM certificates when
interacting with a OneDev server using a self-signed certificate or private
CA.

## Project and repository context

Commands infer the current project from the working directory when available.
For commands supporting `--working-dir`, that flag selects the local directory
used for inference or file operations; it does not specify a server project.
Use `--project` where supported, or a qualified entity reference, to select a
project explicitly.

Issue, pull request, and build API commands can run outside a checkout:

- Use a qualified reference such as `myproject#123` or `PROJ-123` for an
  existing issue, pull request, or build. Bare numbers and `#123` require a
  current project.
- List commands use `--project`, the inferred current project, or all
  accessible projects when neither is available. When listing globally,
  references inside `--query` must also be qualified, for example
  `fixed in pull request "myproject#42"`.
- Creating an issue requires `--project` when no current project is available.
- Creating a pull request or querying its title/description or commit message
  requirements requires `--source-project` and `--source-branch` outside a
  checkout. The target defaults to the source project's upstream project for
  forks, or the source project otherwise.
- Running a build against a branch or tag accepts `--project` and requires it
  when no current project is available. `--project` cannot be combined with
  `--local`.

These commands always require a local repository:

| Command | Required repository context |
|---------|-----------------------------|
| `tod issue checkout <ref>` | The issue's project. |
| `tod pr checkout <ref>` | The PR's source or target project; checkout switches to its source project when needed. |
| `tod issue current-reference` | A Git branch whose name identifies the issue number. |
| `tod pr current-reference` | The current OneDev project and Git branch. |
| `tod project current`, `tod remote` | A Git remote from which the OneDev project can be inferred. |
| `tod build run --local` | The local OneDev repository and changes to submit. |
| `tod build check-spec` | The OneDev repository containing `.onedev-buildspec.yml`. |
| `tod get-commit-message-requirement` | The current OneDev project, even when `--branch` is supplied. |

`tod issue create-branch` only creates or finds the server branch, so it can
run outside a checkout or target another project with a qualified reference.
`tod issue checkout` also creates the server branch when needed, with or
without `--for-write`; that flag adds a write-permission check.

When no current project can be inferred, API requests omit `currentProject`.
The server reports missing context for unqualified references, issue creation
without `--project`, and PR creation/requirement queries without a source
project. Two checks happen locally: branch/tag builds reject a missing project,
and PR creation/requirement queries attempt Git branch inference when
`--source-branch` is omitted. Invalid working directories, Git configuration
errors, and server/authentication failures remain errors.

### Command syntax

Use `tod build run` and `tod pr checkout`. The former top-level `tod run` and
`tod checkout` aliases have been removed.

Top-level groups:

- [`tod issue`](#tod-issue)
- [`tod pr`](#tod-pr)
- [`tod code-comment`](#tod-code-comment)
- [`tod build`](#tod-build)
- [`tod project`](#tod-project)
- [`tod config`](#tod-config)
- [misc: `get-login-name`, `get-unix-timestamp`, `remote`, `get-valid-labels`, `get-commit-message-requirement`, `download`, `version`](#miscellaneous)

Commands that take a `<ref>`/`<source ref>`/`<target ref>` argument accept
`<n>`, `#<n>`, `<project>#<n>`, or `<project-key>-<n>` (e.g. `123`, `#123`,
`myproject#123`, `PROJ-123`). The subcommand (`issue`, `pr`, or `build`)
determines the entity type.

## `tod issue`

| Command | Description |
|---------|-------------|
| `tod issue list` | Query issues. Flags: `--project`, `--query`, `--offset`, `--count`. |
| `tod issue get <ref>` | Detail information of a issue except comments. |
| `tod issue get-comments <ref>` | List comments on an issue. |
| `tod issue create <title>` | Create a new issue. Flags: `--description`, `--project`, `--field key=value` (repeatable), `--iteration` (repeatable), `--own-estimated-time`, `--confidential`. |
| `tod issue edit <ref>` | Edit an existing issue. Flags: `--title`, `--description`, `--field key=value` (repeatable), `--iteration` (repeatable), `--own-estimated-time`, `--confidential`. |
| `tod issue change-state <ref> <state>` | Change state. Flags: `--comment` (Markdown), `--field key=value` (repeatable). |
| `tod issue link <source ref> <link-name> <target ref>` | Add `<target ref>` as `<link-name>` of `<source ref>` (run `tod issue get-valid-links` for valid link names). |
| `tod issue add-comment <ref> <content>` | Add a Markdown comment. |
| `tod issue log-work <ref> <hours>` | Record time spent (`<hours>` must be a positive integer). Flag: `--comment` (Markdown). |
| `tod issue create-branch <ref>` | Create the issue branch on the server if it does not exist, and print the branch name. No checkout required; use a qualified reference without a current project. |
| `tod issue checkout <ref>` | Check out the issue branch into the working directory, set up remote tracking, and update already retrieved submodules. Flag: `--for-write`. |
| `tod issue current-reference` | Print the issue number inferred from the current branch (matched against `[<prefix>/]issue-<n>[-<suffix>]`). Fails if no issue number can be inferred. |
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
| `tod pr get-builds <ref>` | List builds of a pull request. |
| `tod pr get-patch <ref>` | Patch. Flag: `--for-code-review`. |
| `tod pr create <title>` | Create a pull request. Flags: `--source-branch` (defaults to current git branch), `--target-branch`, `--description`, `--source-project`, `--target-project`, `--assignee`, `--reviewer`, `--label`, `--merge-strategy`. |
| `tod pr get-title-and-description-requirement` | Get pull request title and description requirement. Flags: `--source-branch` (defaults to current git branch), `--target-branch`, `--source-project`, `--target-project`. |
| `tod pr get-commit-message-requirement` | Get pull request commit message requirement. Flags: `--source-branch` (defaults to current git branch), `--target-branch`, `--source-project`, `--target-project`. |
| `tod pr edit <ref>` | Edit a pull request. Flags: `--title`, `--description`, `--assignee`, `--add-reviewer`, `--remove-reviewer`, `--label`, `--merge-strategy`, `--auto-merge`, `--auto-merge-commit-message`. |
| `tod pr approve <ref>` | Approve a pull request as a pending reviewer. Flag: `--summary` (Markdown review summary). |
| `tod pr request-changes <ref>` | Request changes on a pull request as a pending reviewer. Flag: `--summary` (Markdown review summary). |
| `tod pr merge <ref>` | Merge a pull request. Flag: `--commit-message`. |
| `tod pr discard <ref>` | Discard (close without merging) a pull request. Flag: `--comment` (Markdown). |
| `tod pr add-comment <ref> <content>` | Add a Markdown comment. |
| `tod pr add-code-comment <ref> <content>` | Add a Markdown code comment to a line range visible on the right side of the PR patch. Flags: `--file` (required), `--from-line` (required, 1-based), `--to-line` (defaults to `--from-line`). |
| `tod pr checkout <ref>` | Check out the pull request into the working directory and update already retrieved submodules. Flag: `--for-write`. |
| `tod pr current-reference` | Print the number of the single open pull request whose source branch is the current git branch. Fails if zero or multiple pull requests match. |
| `tod pr get-query-description` | Print the OneDev pull request query DSL description (syntax reference for `--query` of `pr list`). |

## `tod code-comment`

Code comment IDs are returned by `tod pr get-code-comments <pr-reference>` (the
`id` field on each entry). These commands use that ID directly and do not
require a repository.

| Command | Description |
|---------|-------------|
| `tod code-comment add-reply <comment-id> <content>` | Add a Markdown reply to a code comment. |
| `tod code-comment resolve <comment-id>` | Mark a code comment as resolved. Flag: `--note` (Markdown). |
| `tod code-comment unresolve <comment-id>` | Mark a code comment as unresolved. Flag: `--note` (Markdown). |

## `tod build`

| Command | Description |
|---------|-------------|
| `tod build list` | Query builds. Flags: `--project`, `--query`, `--offset`, `--count`. |
| `tod build get <ref>` | Detail information of a build. |
| `tod build get-log <ref>` | Build log. |
| `tod build get-code-problems <ref> <report-name> <severity-level>` | Found code problems for a build with or higher than the specified severity. Severity level must be one of `CRITICAL`, `HIGH`, `MEDIUM`, or `LOW`. |
| `tod build get-unit-test-report <ref> <report-name>` | Unit test report data. Use `--artifact <path>` to download a report artifact instead. |
| `tod build get-changes-since-success <ref>` | Patch between the build's commit and the previous successful similar build. |
| `tod build run <job-name>` | Run a job against a branch, tag, or local changes. Flags: exactly one of `--branch`, `--tag`, or `--local`; `--project` (branch/tag only); `-p key=value`. Streams the log. |
| `tod build get-spec-schema` | Print the build spec YAML definition. |
| `tod build check-spec` | Validate (and upgrade) `.onedev-buildspec.yml` in the working directory. |
| `tod build get-query-description` | Print the OneDev build query DSL description (syntax reference for `--query` of `build list`). |

## `tod project`

| Command | Description |
|---------|-------------|
| `tod project list` | Query accessible projects. Flags: `--query`, `--offset`, `--count`. |
| `tod project current` | Print the OneDev project inferred from the working directory. |
| `tod project get <project-path>` | Print info of the specified project. |
| `tod project get-query-description` | Print the OneDev project query DSL description (syntax reference for `--query` of `project list`). |

## `tod config`

Create or inspect the tod config file (`server-url`, `access-token`, and
optional `trust-certs-file`). The `ONEDEV_SERVER_URL`, `ONEDEV_ACCESS_TOKEN`,
and `ONEDEV_TRUST_CERTS_FILE` environment variables override these properties.
These commands run even when the config file is missing or invalid.

| Command | Description |
|---------|-------------|
| `tod config set` | Create or update the config file interactively. You are prompted for each property in turn: the current server URL is shown in `[brackets]` as a default (press Enter to keep, or type to replace), and the access-token prompt is always blank (press Enter to keep the existing token, or type a new one). For non-interactive single-property updates, use `tod config set <property name> <property value>`. The file is written with mode `0600` to whichever of `$XDG_CONFIG_HOME/tod/config` or `~/.config/tod/config` already exists, otherwise to `~/.config/tod/config` (or `$XDG_CONFIG_HOME/tod/config` when set). |
| `tod config set <property name> <property value>` | Set one config property. Property name must be `server-url`, `access-token`, or `trust-certs-file`. The trust certs file contains one or more Base64 encoded PEM certificates beginning with `-----BEGIN CERTIFICATE-----` and ending with `-----END CERTIFICATE-----`. |
| `tod config get [property name]` | Print the active configuration or one config property. The access token is always redacted. |
| `tod config path` | Print the path of the config file (the first match in the search order, or the default destination if no file exists yet). |

## Miscellaneous

| Command | Description |
|---------|-------------|
| `tod get-login-name [--user <name>]` | Print the login name of the current user (or `--user`). |
| `tod get-unix-timestamp <description>` | Convert a natural-language description (e.g. `today`, `next month`, `2025-01-01 12:00:00`) to a Unix timestamp in milliseconds. |
| `tod remote` | Print the git remote that points at the inferred OneDev project. |
| `tod get-valid-labels` | Print valid label names for this OneDev server (use with `--label` in `tod pr create`). |
| `tod get-commit-message-requirement [--branch <branch>]` | Print the commit message requirement for a branch in current project (inferred from the working directory). Branch defaults to the current git branch. Prints nothing when the branch has no commit message requirement configured. |
| `tod download <resource-url> <output-file>` | Download a resource (image, file, etc.) referenced in markdown. The resource URL is the original URL from the markdown without modification. Relative URLs are resolved against `server-url`; authentication uses `access-token`. |
| `tod version` | Print the tod version. |
