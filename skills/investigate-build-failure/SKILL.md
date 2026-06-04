---
name: investigate-build-failure
description: Investigate why a OneDev build failed or misbehaved. Use when the user asks to debug, diagnose, or triage a failing or suspicious OneDev build.
---

# Investigate OneDev build failure

Investigate a failing OneDev build using `tod build` commands and files in
the current workspace.

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config set` if needed).
- The current working directory is inside a git repository pointing at the
  OneDev project that owns the build (or the build uses a reference that
  includes the project, e.g. `42`, `myproject#42`)

## Stop on error

**The steps below are sequential and each one depends on the previous
one succeeding.** If any command in the workflow fails — non-zero exit
code, an error message on stderr, empty output where output is
required, or a precondition check (e.g. clean working directory) that
does not hold — you **must**:

1. **Immediately stop the investigation.** Do not run any later
   investigation step, do not "try the next thing anyway", do not
   attempt to repair state beyond what the step itself describes, and
   do not silently retry.
2. **Restore the previous branch if required.** If step 2 has already
   checked out the build commit, perform only the restoration described
   in step 7 before returning to the user.
3. **Surface the exact error to the user** (the command that failed,
   its stderr/stdout, and which step it belongs to).
4. **Wait for the user** to either fix the underlying problem and ask
   you to re-run the skill, or tell you how to proceed.

The rule applies to **every** step, including ones that do not spell it
out explicitly.

## Workflow

Given a `<build-reference>` (e.g. `789`, `#789`, `myproject#789`, or `PROJ-789`):

1. **Get build detail** — overall status, job name, commit, trigger:
   ```bash
   tod build get <build-reference>
   ```
   Save `commitHash` as `<build-commit>`.
2. **Fetch and check out the build commit.** Use a temporary detached
   checkout so workspace files match the version used by the build.

   a. Require a clean working directory:
      ```bash
      git status --porcelain
      ```
      Any output means there are uncommitted changes; stop and ask the user
      to commit or stash them before re-running the skill.

   b. Identify the OneDev remote, fetch the build commit, and check it out
      without moving or creating a local branch:
      ```bash
      tod remote
      git fetch <remote> <build-commit>
      git checkout --detach <build-commit>
      ```

   c. Verify the checkout:
      ```bash
      git rev-parse HEAD
      ```
      The output must equal `<build-commit>`. Otherwise stop.
3. **Get the build log** — scan for errors and the exact failing step:
   ```bash
   tod build get-log <build-reference>
   ```
4. **Inspect or search workspace files as necessary.** Follow references
   from the log to relevant files or symbols. Inspect
   `.onedev-buildspec.yml` when the failure may involve job configuration.
5. **Look at recent changes** — compare the failing build's commit against
   the previous successful similar build to see what changed:
   ```bash
   tod build get-changes-since-success <build-reference>
   ```
6. **Form a hypothesis** — combine the log output, the referenced source,
   and the recent diff to identify the likely cause. Call out specific lines
   in the log and specific hunks in the diff when explaining the failure.
7. **Restore the previous branch.** Once step 2 has checked out the build
   commit, always attempt this restoration before returning to the user,
   including when a later investigation step fails.

   a. Determine whether a previous branch exists:
      ```bash
      git rev-parse --abbrev-ref @{-1} 2>/dev/null
      ```
      Save non-empty output as `<previous-branch>`. If the command fails or
      produces empty output, leave the checkout on the build commit and tell
      the user that restoration was not possible.

   b. Switch back to the previous branch:
      ```bash
      git checkout <previous-branch>
      ```
      Do not delete branches or discard files as part of restoration.
