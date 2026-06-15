---
name: submit-issue-work
description: Submit completed work for a OneDev issue. Use when the user asks to submit, complete, or finish issue work.
---

# Submit work on a OneDev issue

Commit and push an issue branch, then update or create its pull request.

## Prerequisites

- `tod` is installed and configured.
- The current repository belongs to the project that owns the issue.

## Stop on error

Run the workflow sequentially. On any command failure, missing required
output, failed precondition, or declined confirmation, stop immediately,
report the command and error, and wait for the user. Do not continue, retry
silently, amend, or force-push.

## Workflow

Given an `<issue-reference>` (e.g. `123`, `#123`, `myproject#123`, or
`PROJ-123`):

1. **Resolve and verify the issue branch.**
   ```bash
   tod issue create-branch <issue-reference>
   git symbolic-ref --short HEAD
   ```
   Save the first output as `<issue-branch>`. The current branch must match it.

2. **Resolve PR context before composing a commit message.**
   ```bash
   tod pr list --query 'includes issue "<issue-reference>" and open'
   ```
   - **One match:** Read it and verify its source project and branch match the
     current project and `<issue-branch>`:
     ```bash
     tod pr get <pr-reference>
     tod project current
     ```
     Record the PR reference, URL, source, and target values.
   - **No matches:** Plan a new PR. Use default `tod pr create` values unless
     the user requested non-default source/target projects, target branch, or
     merge strategy.
   - **Multiple matches:** Ask the user which PR to use, then stop.

3. **Commit pending work.**
   ```bash
   git status --porcelain
   ```
   If dirty, generate a message using:
   ```bash
   tod get-commit-message-requirement
   tod pr get-commit-message-requirement
   ```
   Pass the existing or planned PR source/target values from step 2 to the
   second command. Inspect the diff, satisfy all non-empty requirements, and
   ask the user to confirm the full message before running:
   ```bash
   git add -A
   git commit -m "<subject>" -m "<body>"
   git status --porcelain
   ```
   The final status must be clean. Skip this step when it was already clean.

4. **Capture and push new commits.**
   ```bash
   tod remote
   git fetch <remote> <issue-branch>
   git branch --set-upstream-to=<remote>/<issue-branch> <issue-branch>
   git log --reverse --pretty=format:'%h %s%n%b%n---' <remote>/<issue-branch>..HEAD
   ```
   Save the remote and commit log. If the log is empty, report that there is
   nothing to submit and stop. Otherwise:
   ```bash
   git push <remote> <issue-branch>
   ```

5. **Update or create the PR.**
   - For an existing PR, the push updated it. Report its reference and URL.
   - For a new PR, read its requirements using the same planned flags:
     ```bash
     tod pr get-title-and-description-requirement
     ```
     Compose a concise title and description from the captured commits,
     validate them, and obtain confirmation before:
     ```bash
     tod pr create "<title>" --description "<description>"
     ```
     Pass the same non-default flags used for the requirement command and
     report the returned PR reference and URL.

6. **Apply deferred OneDev changes.** After the push and PR update/create
   succeed, apply any deferred comments or state changes. Obtain explicit
   consent before each state-changing `tod` command.

7. **Restore the previous branch.**
   ```bash
   git rev-parse --abbrev-ref @{-1} 2>/dev/null
   ```
   If this returns a branch different from `<issue-branch>`, run:
   ```bash
   git checkout <previous-branch>
   git branch -D <issue-branch>
   ```
   Otherwise leave the checkout on `<issue-branch>`. Never remove the remote
   issue branch.
