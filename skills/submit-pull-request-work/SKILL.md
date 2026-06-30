---
name: submit-pull-request-work
description: Submit completed work for an existing OneDev pull request. Use when the user asks to submit, complete, or finish PR work.
---

# Submit pull request work

Submit code changes and/or saved comments for an existing pull request. This
workflow does not create a pull request.

## Prerequisites

- `tod` is installed and configured.
- The current repository belongs to the PR's source or target project.

## Session handoff

This workflow pairs with `work-on-pull-request`. At the start, recover
`<saved-pr-actions>` from the **same chat session**:

1. **From a prior `work-on-pull-request` run** -- use the exact drafted actions
   presented or amended earlier in this session, including comment text and
   parameters (`comment-id`, file, line range, approve/request-changes, merge,
   commit message, etc.).
2. **From the user's submit prompt** -- if the user supplies or revises actions
   when asking to submit, treat that as `<saved-pr-actions>`.
3. **Otherwise** -- `<saved-pr-actions>` is empty.

`<saved-pr-actions>` is session state, not a file on disk and not discussion
already on OneDev. Step 6 applies these deferred drafts.

## Stop on error

Run the workflow sequentially. On any command failure, missing required
output, failed precondition, or declined confirmation, stop immediately,
report the command and error, and wait for the user. Do not continue, retry
silently, amend, or force-push.

## If not submitting

If you decide not to run submission for any reason, make sure to create a PR
comment explaining why the work was not submitted:
```bash
tod pr add-comment <pr-reference> '<reason>'
```

## Shell quoting for authored text

When passing authored text such as Markdown comments, review notes, summaries,
or commit messages to `tod` from a shell command, quote it so the shell
preserves it literally. Prefer a single-quoted argument, and escape any literal
single quote inside the text as `'\''`. Do not wrap text containing backticks in
double quotes, as the shell will treat backticks as command substitution before
`tod` receives the text.

## Workflow

Given an optional `<pr-reference>` (e.g. `42`, `#42`, `myproject#42`, or
`PROJ-42`):

1. **Resolve the PR reference.** If the user prompt or session context already
   provides `<pr-reference>`, use it. Otherwise derive it from the working
   directory:
   ```bash
   tod pr current-reference
   ```
   Save non-empty output as `<pr-reference>`. If the output is empty, stop
   and report that the PR reference could not be derived.

2. **Check whether code can be submitted.**
   ```bash
   git symbolic-ref --short HEAD 2>/dev/null
   ```
   Save successful output as `<current-branch>`. If there is no branch, apply
   `<saved-pr-actions>` using the routing in step 6, and continue with rest
   steps.

3. **Verify the current branch.**
   ```bash
   tod pr get <pr-reference>
   tod project current
   tod remote
   git fetch <remote> <source-branch>
   git rev-parse --abbrev-ref <current-branch>@{upstream}
   ```
   From `tod pr get`, note `<source-project>`, `<target-project>`,
   `<source-branch>`, `<target-branch>`, and the PR reference/URL. Save the
   other outputs as `<current-project>`, `<remote>`, and `<upstream>`. If the PR
   is not open, stop and tell the user. Verify that `<current-project>` equals
   `<source-project>`, `<current-branch>` equals `<source-branch>`, and
   `<upstream>` equals `<remote>/<source-branch>`. If any check fails, report
   the mismatch and stop.

4. **Verify the working copy is clean.**
   ```bash
   git status --porcelain
   ```
   If the output is non-empty, report that code submission expects committed
   work on `<source-branch>` and stop. Do not stage, commit, amend, or discard
   changes in this workflow.

5. **Push outstanding commits.**
   ```bash
   git log --reverse --pretty=format:'%h %s%n%b%n---' <remote>/<source-branch>..HEAD
   ```
   Save the output as `<commits-to-push>`. If it is empty, skip the push and
   continue to deferred comments or state changes. Otherwise:
   ```bash
   git push <remote> <source-branch>
   ```
   Report the PR reference/URL from step 3. When you pushed, the existing pull
   request now includes the new commits; do **not** run `tod pr create`.

6. **Apply deferred OneDev changes.** Apply `<saved-pr-actions>` from
   **Session handoff**, whether or not this workflow submitted code. If code
   submission started and then failed, do **not** apply saved actions.

   - If `<saved-pr-actions>` is non-empty, apply every action -- do not skip
     because similar text already appears elsewhere on the PR.
   - If `<saved-pr-actions>` is empty and no code was submitted, report that
     there is nothing to submit.

   For each action in `<saved-pr-actions>`, use the command that matches where
   the discussion lives:
   - New line-anchored finding -> `tod pr add-code-comment <pr-reference> '<comment>' --file <path> --from-line <line> [--to-line <line>]`
   - General PR feedback -> `tod pr add-comment <pr-reference> '<reply>'`
   - Line-anchored thread -> `tod code-comment add-reply <comment-id> '<reply>'`
   - Outstanding concern addressed in code -> `tod code-comment resolve <comment-id> --note '<why>'` when appropriate
   - Concern stated addressed but not actually addressed in code -> `tod code-comment unresolve <comment-id> --note '<why>'` when appropriate
   - Reviewer outcome -> `tod pr approve <pr-reference>` or
     `tod pr request-changes <pr-reference>` when the saved action is a
     pending-reviewer state change; include `--summary '<summary>'` when the
     saved outcome has summary text
   - Merge outcome -> `tod pr merge <pr-reference>`; include
     `--commit-message '<commit-message>'` when saved

7. **Restore the previous branch and clean up the current branch if applicable.**
   ```bash
   git rev-parse --abbrev-ref @{-1} 2>/dev/null
   ```
   Save successful output as `<previous-branch>`. If the command fails or returns
   an empty value, stop.

   - If `<current-branch>` was not recorded in step 2:
     ```bash
     git checkout <previous-branch>
     ```
   - If `<current-branch>` differs from `<previous-branch>`:
     ```bash
     git checkout <previous-branch>
     git branch -d <current-branch>
     ```
   - If `<current-branch>` equals `<previous-branch>`, do nothing.
