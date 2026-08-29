---
name: submit-issue-work
description: Submit completed work for a OneDev issue. Use when the user asks to submit, complete, or finish issue work.
---

# Submit work on a OneDev issue

Submit code changes and/or saved comments for an issue.

## Prerequisites

- `tod` is installed and configured.
- The current repository belongs to the issue's project.

## Session handoff

This workflow pairs with `work-on-issue`. At the start, recover
`<saved-issue-comments>` from the **same chat session**:

1. **From a prior `work-on-issue` run** -- use the exact drafted comment text
   presented or amended earlier in this session.
2. **From the user's submit prompt** -- if the user supplies or revises comment
   text when asking to submit, treat that as `<saved-issue-comments>`.
3. **Otherwise** -- `<saved-issue-comments>` is empty.

`<saved-issue-comments>` is session state, not a file on disk and not comments
already on OneDev. Step 7 posts these deferred drafts.

## Aborting the workflow

Run the workflow sequentially. Before aborting it for any reason, always follow
this section. If current user is an AI user and `<issue-reference>` is known,
immediately run
`tod issue add-comment <issue-reference> '<reason>'` to explain the stop reason.
Report the reason, including the command and error when applicable, and stop.


## Interactive questions

If current user is an AI user, do not ask the
current user for direction in this workflow. When you would otherwise ask a
question, post a concise issue comment explaining the blocker or needed
decision, then stop:
```bash
tod issue add-comment <issue-reference> '<reason>'
```

## Shell quoting for authored text

When passing authored text such as Markdown comments, commit messages, PR
titles, or PR descriptions to a shell command, quote it so the shell preserves
it literally. Prefer a single-quoted argument, and escape any literal single
quote inside the text as `'\''`. Do not wrap text containing backticks in double
quotes, as the shell will treat backticks as command substitution before the
command receives the text.

## Markdown references for issue, pull request, or build

In authored OneDev Markdown, write an entity reference as `<type> <reference>`,
such as `PR #42`, `issue acme/web#123`, or `build ACMEWEB-7`. With no type the 
reference means an issue. `<reference>` is `#123` for an entity in the current 
project, `path/to/project#123` for one in another project, or `PROJECTKEY-123`, 
which works from any project as long as the project has a key defined. These 
forms differ from `tod` command arguments.

## Workflow

Given an optional `<issue-reference>` (e.g. `123`, `#123`, `myproject#123`,
or `PROJ-123`):

1. **Resolve the issue reference.** If the user prompt or session context
   already provides `<issue-reference>`, use it. Otherwise derive it from the
   working directory:
   ```bash
   tod issue current-reference
   ```
   Save non-empty output as `<issue-reference>`. If the output is empty, stop
   and report that the issue reference could not be derived.

2. **Check whether code can be submitted.**
   ```bash
   git symbolic-ref --short HEAD 2>/dev/null
   ```
   Save successful output as `<current-branch>`. If there is no branch, skip to
   step 7 to apply saved issue comments, and continue with rest steps.

3. **Verify the current branch.**
   ```bash
   tod issue get <issue-reference>
   tod project current
   tod remote
   git fetch <remote> <issue-branch>
   git rev-parse --abbrev-ref <current-branch>@{upstream}
   ```
   Save the issue detail as `<issue>` and get `<issue-branch>` from that
   response. If the branch is missing from the issue detail, report that the
   issue does not have a branch on the server yet and stop. Save the other
   outputs as `<current-project>`, `<remote>`, and `<upstream>`. Verify that
   `<current-project>` equals the issue project, `<current-branch>` equals
   `<issue-branch>`, and `<upstream>` equals `<remote>/<issue-branch>`. If any
   check fails, report the mismatch and stop.

4. **Find the PR, or plan to create one.**
   ```bash
   tod pr list --query 'includes issue "<issue-reference>" and open'
   ```
   - **One match:** Read it and verify its source project and branch match the
     verified issue project and branch:
     ```bash
     tod pr get <pr-reference>
     ```
     Record the PR reference, URL, source, and target values.
   - **No matches:** Plan to create a PR. Use user-provided target project and
     target branch values first. For any missing target value, use the
     corresponding default from the issue detail already read in step 3. If a
     required target value is still missing, report that work is not submitted
     because that value is missing, and stop. Otherwise record the planned PR
     target values.
   - **Multiple matches:** If the user prompt indicates the current user is an
     AI user, post an issue comment explaining that multiple open PRs include
     the issue and submission needs one PR to be selected, then stop.
     Otherwise, ask the user which PR to use, then stop.

5. **Commit changed submodules before the parent repository.**
   ```bash
   git status --porcelain
   ```
   If the output is empty, skip this step. Otherwise process changed
   submodules deepest-first. For each one, after its changed descendants have
   been processed, inspect its final diff and verify its intended branch and
   upstream. If it is dirty, run `tod get-commit-message-requirement` inside
   it, compose a compliant message here (do not use the
   `generate-commit-message` skill), commit, push commits not yet on the
   upstream, and verify the worktree is clean.

   If a submodule push is rejected specifically because it is not a
   fast-forward, pull and merge its upstream branch, then retry once:
   ```bash
   git pull --no-rebase <upstream-remote> <upstream-branch>
   git push <upstream-remote> <upstream-branch>
   ```
   Resolve any merge conflicts, complete the merge commit, and verify the
   submodule worktree is clean before retrying. Keep the merged tip checked out
   so its parent records the resulting gitlink. Do not force-push. Stop on any
   other submodule commit or push failure, or if the retry fails. A parent
   gitlink must not be committed until its referenced submodule commit is
   available remotely.

   After all changed submodules are pushed, inspect the parent repository's
   final diff and compose its message from:
   ```bash
   tod get-commit-message-requirement
   tod pr get-commit-message-requirement --target-project <target-project> --target-branch <target-branch>
   ```
   Pass the existing or planned PR target values from step 4, satisfy all
   non-empty requirements, then commit the parent repository:
   ```bash
   git add -A
   git commit -m '<subject>' -m '<body>'
   git status --porcelain
   ```
   The final status must be clean.
   
6. **Push outstanding commits and create a PR when needed.**
   ```bash
   git log --reverse --pretty=format:'%h %s%n%b%n---' <remote>/<issue-branch>..HEAD
   ```
   Save the output as `<commits-to-push>`. If it is empty, skip the push and
   PR creation and continue to deferred comments or state changes. Otherwise:
   ```bash
   git push <remote> <issue-branch>
   ```

   - For an existing PR, the push updated it.
   - If no PR was found in step 4, read the new PR requirements using the same
     planned flags:
     ```bash
     tod pr get-title-and-description-requirement --target-project <target-project> --target-branch <target-branch>
     ```
     Compose a concise title and description from the captured commits,
     validate them, then run:
     ```bash
     tod pr create '<title>' --description '<description>' --target-project <target-project> --target-branch <target-branch>
     ```
     Pass the resolved target project and target branch values. Save the
     returned PR reference and URL.

   After the existing PR is updated or the new PR is created:

   - If the current user is an AI user, draft a concise PR mention using the
     forms above. If `<saved-issue-comments>` contains exactly one comment,
     combine the PR mention with it. Otherwise, append the PR mention to
     `<saved-issue-comments>` as a separate comment.
   - Otherwise, report the PR reference and URL to the current user.

7. **Apply deferred OneDev changes.** Post `<saved-issue-comments>`, whether or
   not this workflow submitted code. If code submission started and then
   failed, do not post saved comments.

   - If `<saved-issue-comments>` is non-empty, post every comment -- do not
     skip because a PR description already covers the same ground.
   - If `<saved-issue-comments>` is empty and no code was submitted, report
     that there is nothing to submit.

   For each comment in `<saved-issue-comments>`, run:
   ```bash
   tod issue add-comment <issue-reference> '<comment>'
   ```

8. **Restore the previous checkout and clean up the current branch if applicable.**
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
