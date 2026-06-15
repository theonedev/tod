---
name: investigate-build-failure
description: Diagnose a failed or suspicious OneDev build. Use when the user asks to investigate, debug, or triage build behavior.
---

# Investigate OneDev build failure

Diagnose a OneDev build from its metadata, log, source, and recent changes.

## Prerequisites

- `tod` is installed and configured.
- The current repository owns the build, or the build reference includes its
  project.

## Stop on error

Run the workflow sequentially. On any command failure, missing required
output, or failed precondition, stop, report the command and error, and wait
for the user. Do not continue or retry silently. If the build commit was
checked out, perform only the final restoration step before stopping.

## Workflow

Given a `<build-reference>` (e.g. `789`, `#789`, `myproject#789`, or `PROJ-789`):

1. **Read build details** such as status, job, commit, and trigger:
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
3. **Read the build log** and identify errors and the failing step:
   ```bash
   tod build get-log <build-reference>
   ```
4. **Inspect or search workspace files as necessary.** Follow references
   from the log to relevant files or symbols. Inspect
   `.onedev-buildspec.yml` when the failure may involve job configuration.
5. **Inspect recent changes** since the previous successful similar build:
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
