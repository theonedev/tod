# TOD - TheOneDev CLI Tool

TOD (**T**he**O**ne**D**ev) is a powerful command-line tool for OneDev 15.1+
that streamlines your development workflow by letting you run CI/CD jobs
against local changes, check out pull requests into a local working
directory, query and edit issues/PRs/builds, and drive all of the above from
AI agents via shipped skill files.

## Features

- **Query and edit OneDev entities** — issues, pull requests, and builds —
  directly from the shell.
- **Run CI/CD jobs against local changes, branches, or tags** with real-time
  log streaming (`tod build run --local`, `--branch`, or `--tag`).
- **Check out pull requests** locally (`tod pr checkout`).
- **Check and migrate `.onedev-buildspec.yml`** to the latest version
  (`tod build check-spec`).
- **Agent skills** under [`skills/`](skills/) that teach Claude Code, Cursor,
  and other SKILL.md-aware agents to drive OneDev workflows via `tod`.
- **Cross-platform support** (Windows, macOS, Linux).

## Installation

To install `tod`, put the binary on your `PATH`.

### Download pre-built binaries

https://code.onedev.io/onedev/tod/~builds?query=%22Job%22+is+%22Release%22

### Build from source

**Requirements:** Go 1.22.1 or higher.

```bash
git clone https://code.onedev.io/onedev/tod.git
cd tod
go build
```

## Configuration

Run `tod config set` to create or update the config file interactively. Each
property is prompted for in turn — the current server URL is shown in
`[brackets]` as a default (press Enter to keep it, or type a new value to
replace it), and the access-token prompt is always blank (press Enter to
keep the existing token, or type a new one to replace it):

```bash
tod config set
# OneDev server URL [https://onedev.example.com]: ...
# OneDev personal access token (press Enter to keep existing): ...
```

For scripts and other non-interactive setups, pass the property name and
value positionally to update one property at a time without prompts:

```bash
tod config set server-url https://onedev.example.com
tod config set access-token your-personal-access-token
```

`tod config get` prints the active configuration (with the token redacted)
and `tod config get <property name>` prints a single property. Property
names are `server-url` and `access-token`. `tod config path` prints the
path being used.

The config file is searched at the following locations (first match wins):

1. `$XDG_CONFIG_HOME/tod/config`
2. `~/.config/tod/config`

It uses INI format and is written with mode `0600`:

```ini
server-url=https://onedev.example.com
access-token=your-personal-access-token
```

## Quick start

```bash
# Run CI job against your uncommitted changes
cd /path/to/onedev-git-repository
tod build run --local ci

# Run ci job against the main branch
tod build run --branch main ci

# Check out pull request PROJ-123 into the current working directory
tod pr checkout PROJ-123

# Query open issues assigned to you
tod issue list --query 'assignee is me and state is "Open"'

# Inspect the most recent failing build for a project
tod build list --query 'not(successful)' --count 1
tod build get <ref>
tod build get-log <ref>
```

See [cli.md](cli.md) for the full command reference.

## Agent skills

TOD ships nine tool-agnostic `SKILL.md` files under [`skills/`](skills/) that
teach AI agents how to drive common OneDev workflows through the CLI:

- `using-tod` — umbrella guide for any OneDev interaction via `tod`
- `edit-build-spec` — create or edit `.onedev-buildspec.yml`
- `investigate-build-failure` — debug a failing build
- `review-pull-request` — perform a structured pull request review
- `generate-commit-message` — compose a commit message that respects the
  project's commit-message requirement and the active issue
- `work-on-issue` — create the issue branch on the server, switch the local
  checkout to it, read the issue, and implement the work
- `submit-issue-work` — commit pending changes, push the issue branch, and
  open a pull request
- `work-on-pull-request` — check out a PR on its source project, address
  review or CI feedback locally, and draft push-dependent replies
- `submit-pull-request-work` — commit pending changes, push to update an
  existing pull request, then post deferred replies so notifications match
  visible commits

See [skills/README.md](skills/README.md) for how to install these into Claude
Code, Codex, Cursor, or any other agent that reads `SKILL.md` files.

## Notes for local CI runs

### Nginx configuration

If OneDev is running behind Nginx, disable HTTP buffering for log streaming:

```nginx
location /~api/streaming {
    proxy_pass http://localhost:6610/~api/streaming;
    proxy_buffering off;
}
```

See the [OneDev Nginx setup documentation](https://docs.onedev.io/administration-guide/reverse-proxy-setup#nginx)
for details.

### Security considerations

If the job accesses job secrets, make sure the authorization field is cleared
to allow all jobs. Setting authorization to allow all branches is not
sufficient — local changes are pushed to a temporal ref that does not belong
to any branch.

### Performance tips

1. **Large repositories** — use an appropriate clone depth in checkout steps
   instead of full history.
2. **External dependencies** — use
   [caching](https://docs.onedev.io/tutorials/cicd/job-cache) for downloads
   and intermediate files.
3. **Build optimization** — cache slow-to-generate intermediate files.

## Contributing

TOD is part of the OneDev ecosystem. For contributions, issues, and feature
requests, visit the [OneDev project](https://code.onedev.io/onedev/tod).

## License

See [license.txt](license.txt).
