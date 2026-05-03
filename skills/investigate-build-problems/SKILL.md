---
name: investigate-build-problems
description: Investigate why a OneDev build failed or misbehaved by reading its metadata, log, referenced files, and recent changes via the `tod` CLI. Use when the user asks to debug, diagnose, or triage a failing or suspicious OneDev build.
---

# Investigate OneDev build problems

This skill walks the agent through a systematic investigation of a failing or
suspicious OneDev build using the `tod build` subcommands.

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config init` if needed).
- The current working directory is inside a git repository pointing at the
  OneDev project that owns the build.

## Workflow

Given a `<build-reference>` (e.g. `#789`, `myproject#789`, or `PROJ-789`):

1. **Get build metadata** — overall status, job name, commit, trigger:
   ```bash
   tod build show <build-reference>
   ```
2. **Get the build log** — scan for errors and the exact failing step:
   ```bash
   tod build log <build-reference>
   ```
3. **Inspect referenced files** — for any file mentioned in the log, fetch
   the exact version used by the build:
   ```bash
   tod build file-content <build-reference> --path <path>
   ```
   Always start with the build spec if there is any doubt about
   configuration:
   ```bash
   tod build file-content <build-reference> --path .onedev-buildspec.yml
   ```
4. **Look at recent changes** — compare the failing build's commit against
   the previous successful similar build to see what changed:
   ```bash
   tod build changes-since-success <build-reference>
   ```
5. **Form a hypothesis** — combine the log output, the referenced source,
   and the recent diff to identify the likely cause. Call out specific lines
   in the log and specific hunks in the diff when explaining the failure.

## Tips

- If the log is very long, scan it from the bottom up — the first error
  message is usually the highest-signal clue.
- Compare the failing job definition in `.onedev-buildspec.yml` (from step 3)
  with any recent changes to that file (step 4) — spec regressions are a
  common cause of sudden failures.
- Use `tod build list --query '...'` if you need context from neighbouring
  builds (for example, to see whether the same job passed on another branch).
