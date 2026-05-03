# TOD - TheOneDev CLI Tool

TOD (**T**he**O**ne**D**ev) is a powerful command-line tool for OneDev 13+
that streamlines your development workflow by letting you run CI/CD jobs
against local changes, check out pull requests into a local working
directory, query and edit issues/PRs/builds, and drive all of the above from
AI agents via shipped skill files.

## Features

- **Query and edit OneDev entities** — issues, pull requests, builds, and
  packages — directly from the shell.
- **Run CI/CD jobs against local changes** without committing or pushing
  (`tod build run-local`).
- **Run jobs against specific branches or tags** with real-time log streaming
  (`tod build run`).
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

Run `tod config init` to create or update the config file interactively:

```bash
tod config init
# OneDev server URL (e.g. https://onedev.example.com): ...
# OneDev personal access token (input is visible): ...
```

Or pass everything as flags for non-interactive setups:

```bash
tod config init \
  --server-url https://onedev.example.com \
  --access-token your-personal-access-token \
  --non-interactive
```

`tod config show` prints the active configuration (with the token redacted),
and `tod config path` prints the path being used.

The config file is searched at the following locations (first match wins):

1. `$XDG_CONFIG_HOME/tod/config`
2. `~/.config/tod/config`
3. `~/.todconfig` (legacy fallback)

It uses INI format and is written with mode `0600`:

```ini
server-url=https://onedev.example.com
access-token=your-personal-access-token
```

## Quick start

```bash
# Run CI against your uncommitted changes
cd /path/to/onedev-git-repository
tod build run-local ci

# Run ci job against the main branch
tod build run --branch main ci

# Check out pull request PROJ-123 into the current working directory
tod pr checkout PROJ-123

# Query open issues assigned to you
tod issue list --query 'assignee is me and state is "Open"'

# Inspect the most recent failing build for a project
tod build list --query 'successful is false' --count 1
tod build show <ref>
tod build log <ref>
```

See [cli.md](cli.md) for the full command reference.

## Agent skills

TOD ships four tool-agnostic `SKILL.md` files under [`skills/`](skills/) that
teach AI agents how to drive common OneDev workflows through the CLI:

- `using-tod` — umbrella guide for any OneDev interaction via `tod`
- `edit-build-spec` — create or edit `.onedev-buildspec.yml`
- `investigate-build-problems` — debug a failing build
- `review-pull-request` — perform a structured pull request review

See [skills.md](skills.md) for how to install these into Claude Code, Cursor,
or any other agent that reads `SKILL.md` files.

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
