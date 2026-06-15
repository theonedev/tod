---
name: submit-pull-request-work
description: Submit completed work for an existing OneDev pull request. Use when the user asks to submit, complete, or finish PR work.
---

# Submit pull request work

Publish local changes to an existing pull request, then apply any OneDev
updates deferred until after the push. This workflow does not create a pull
request.

## Prerequisites

- `tod` is installed and configured.
- The current repository is the PR's source project.

## Stop on error

Run the workflow sequentially. On any command failure, missing required
output, failed precondition, or declined confirmation, stop immediately,
report the command and error, and wait for the user. Do not continue, retry
silently, amend, or force-push.

## Workflow

Resolve a `<pr-reference>` (e.g. `42`, `#42`, `myproject#42`, or
`PROJ-42`) from the user's message. When they did not name one, query for an
open PR whose source branch is the current branch:
```bash
git symbolic-ref --short HEAD
tod pr list --query 'open and "Source Branch" is "<current-branch>"'
```
Use the sole match. Ask the user when there are zero or multiple matches.

1. **Read the pull request and confirm the source project.**
   ```bash
   tod pr get <pr-reference>
   tod project current
   ```
   From `tod pr get`, note `<source-project>`, `<target-project>`,
   `<source-branch>`, `<target-branch>`, and the PR reference/URL. The
   output of `tod project current` must equal
   `<source-project>`. If it does not, stop and tell the user the checkout
   is in the wrong project. If the PR is not open, stop and tell the user.

2. **Prepare the local source branch.**
   ```bash
   git symbolic-ref --short HEAD
   ```
   If the output does **not** equal `<source-branch>`, stop and tell the
   user to check out the PR source branch before submitting.

3. **Commit any pending work.** Check whether the working copy is clean:
   ```bash
   git status --porcelain
   ```
   - **Empty output** → working copy is clean, skip to step 4.
   - **Non-empty output** → there are uncommitted changes. Generate the commit
     message as follows:
     1. Run `tod get-commit-message-requirement`.
     2. Run `tod pr get-commit-message-requirement` with the source and target
        project and branch values from step 1.
     3. Inspect `git diff` and `git status`, then compose a concise imperative
        subject and a body explaining why the change was made.
     4. Validate the message against every non-empty requirement.

     Show the proposed message to the user and only after explicit confirmation
     run:
     ```bash
     git add -A
     git commit -m "<subject>" -m "<body>"
     ```
     (Use a `HEREDOC`-style invocation if the message contains multiple
     paragraphs or special characters.) After the commit, re-run
     `git status --porcelain` to confirm the working copy is now clean.

4. **Set up remote tracking and push.**

   a. Identify the OneDev remote:
      ```bash
      tod remote
      ```
      Save the output as `<remote>`.

   b. Make sure the local branch tracks the same-named branch on the
      server. This is idempotent:
      ```bash
      git fetch <remote> <source-branch>
      git branch --set-upstream-to=<remote>/<source-branch> <source-branch>
      ```

   c. **Capture the commits to be pushed before pushing**, because after
      the push the local branch and its upstream are identical:
      ```bash
      git log --reverse --pretty=format:'%h %s%n%b%n---' <remote>/<source-branch>..HEAD
      ```
      Save the output as `<commits-to-push>`. **If the output is empty**,
      there are no new commits to push — skip step 4d. If this session also
      has **no** deferred OneDev replies or resolves to post in step 6, stop
      and tell the user there are no new commits to push. Otherwise continue
      without pushing.

   d. When `<commits-to-push>` is non-empty, push the source branch:
      ```bash
      git push <remote> <source-branch>
      ```

5. **Report the update.** Surface `<commits-to-push>` (when non-empty) and
   the PR reference/URL from step 1 to the user. When you pushed, the existing
   pull request now includes the new commits; do **not** run `tod pr create`.

6. **Post deferred follow-up on OneDev.** If this session has replies or
   resolves that depend on pushed code, post them **now** — only after step 4d
   succeeded when there was a push, or immediately when `<commits-to-push>` was
   empty but those fixes are already on the server.

   Before each state-changing `tod` command, show the exact planned action and
   obtain explicit user consent. Match the channel:
   - General PR feedback → `tod pr add-comment <pr-reference> "<reply>"`
   - Line-anchored thread → `tod code-comment add-reply <comment-id> "<reply>"`
   - Concern addressed in code → `tod code-comment resolve <comment-id> --note "<why>"` when appropriate
   - Issue-side discussion → `tod issue add-comment <issue-ref> "<reply>"` when the thread lives on the issue

   If there are no deferred writes for this session, skip this step. If a push
   was required but step 4d did not run or failed, do **not** post deferred
   replies.

7. **Return to the previous branch and remove the local source branch.** Only
   after steps 4–6 complete successfully:

   a. Determine whether a previous branch exists:
      ```bash
      git rev-parse --abbrev-ref @{-1} 2>/dev/null
      ```
      Save non-empty output as `<previous-branch>`. If the command fails or
      produces empty output, skip the rest of this step — leave the checkout
      on `<source-branch>`.

   b. If `<previous-branch>` equals `<source-branch>`, skip the rest of this
      step — there is nowhere else to switch to.

   c. Switch back and delete the local source branch. The remote branch on
      `<remote>` stays for the open pull request:
      ```bash
      git checkout <previous-branch>
      git branch -D <source-branch>
      ```

   Tell the user the checkout is now on `<previous-branch>` and the local
   `<source-branch>` was removed.
