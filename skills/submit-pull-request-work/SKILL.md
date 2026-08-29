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
   etc.).
2. **From the user's submit prompt** -- if the user supplies or revises actions
   when asking to submit, treat that as `<saved-pr-actions>`.
3. **Otherwise** -- `<saved-pr-actions>` is empty.

`<saved-pr-actions>` is session state, not a file on disk and not discussion
already on OneDev. Step 5 applies these deferred drafts.

## Aborting the workflow

Run the workflow sequentially. Before aborting it for any reason, always follow
this section. If the current user is an AI user and `<pr-reference>` is known,
immediately run
`tod pr add-comment <pr-reference> '<reason>'` to explain the stop reason.
Report the reason, including the command and error when applicable, and stop.


## Interactive questions

If the current user is an AI user, do not ask the
current user for direction in this workflow. When you would otherwise ask a
question, post a concise PR comment explaining the blocker or needed decision,
then stop:
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

## Markdown references for issue, pull request, or build

In authored OneDev Markdown, write an entity reference as `<type> <reference>`,
such as `PR #42`, `issue acme/web#123`, or `build ACMEWEB-7`. With no type, the
reference means an issue. `<reference>` is `#123` for an entity in the current
project, `path/to/project#123` for one in another project, or `PROJECTKEY-123`,
which works from any project as long as the project has a key defined. Keep the
type and reference separated by one space with nothing between them. These
forms differ from `tod` command arguments.

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
   `<saved-pr-actions>` by skipping to step 5.

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

4. **Commit and push changes from the deepest submodules outward.**
   ```bash
   git status --porcelain
   ```
   Inspect dirty retrieved submodules recursively and process them
   deepest-first.

   For each dirty submodule, first process its descendants, then inspect its
   final diff and verify its intended branch and upstream. Run
   `tod get-commit-message-requirement` inside it, compose a compliant
   message here (do not use the `generate-commit-message` skill), stage and
   commit its changes, and verify its worktree is clean. Push the new commits
   to its upstream.

   If the push is rejected specifically because it is not a fast-forward,
   merge the upstream branch and retry once:
   ```bash
   git pull --no-rebase <upstream-remote> <upstream-branch>
   git push <upstream-remote> <upstream-branch>
   ```
   Resolve conflicts and complete the merge commit before retrying. Do not
   force-push. Stop if a commit fails, another push error occurs, or the retry
   fails.

   After all dirty submodules are committed and pushed, inspect the source
   repository's final diff. If it is dirty, compose its message from:
   ```bash
   tod get-commit-message-requirement
   tod pr get-commit-message-requirement --target-project <target-project> --target-branch <target-branch>
   ```
   Satisfy every non-empty requirement, then commit the source repository:
   ```bash
   git add -A
   git commit -m '<subject>' -m '<body>'
   git status --porcelain
   ```
   The final status must be clean.

   Push the source repository:
   ```bash
   git push <remote> <source-branch>
   ```
   Stop on failure; do not force-push.

5. **Apply deferred OneDev changes.** Apply `<saved-pr-actions>` from
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
   - Merge outcome -> inspect `tod pr get <pr-reference>`. If its merge strategy
     is `SQUASH_SOURCE_BRANCH_COMMITS`, read:
     ```bash
     tod pr get-commit-message-requirement --target-project <target-project> --target-branch <target-branch>
     tod issue list --query 'fixed in pull request "<pr-reference>"'
     ```
     Compose the commit message here from the PR title, description, and fixed
     issues; satisfy every non-empty requirement without using the
     `generate-commit-message` skill. Then run `tod pr merge <pr-reference>
     --commit-message '<commit-message>'`. For other merge strategies, run
     `tod pr merge <pr-reference>`.

6. **Restore the previous checkout and clean up the current branch if applicable.**
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

   After checking out `<previous-branch>`, restore every retrieved submodule
   worktree to the commit recorded by the restored parent checkout:
   ```bash
   git submodule update --recursive
   ```
   Do not pass `--init`: submodules that have not been retrieved must remain
   unretrieved. Verify that the parent repository working copy is clean.
