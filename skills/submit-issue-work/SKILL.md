---
name: submit-issue-work
description: Submit completed work on a OneDev issue. Use when the user asks
  to submit, complete, or finish work on a OneDev issue, such as "submit work
  123", "complete work on PROJ-7", or "finish issue myproject#42".
---

# Submit work on a OneDev issue

This skill takes a OneDev issue from "implementation done in the working
copy" to "pull request opened or updated on the server". It assumes the
agent has already implemented the change on the issue branch (typically
after running the `work-on-issue` skill).

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config set` if needed).
- The current working directory is inside a git repository pointing at
  the OneDev project that owns the issue.

## Stop on error

**The steps below are sequential and each one depends on the previous
one succeeding.** If any command in the workflow fails — non-zero exit
code, an error message on stderr, empty output where output is
required, a failed precondition check (e.g. "wrong branch", "no new
commits to push"), or the user declining to confirm a commit message
or PR description — you **must**:

1. **Immediately stop the workflow.** Do not run any later step, do
   not "try the next thing anyway" (for example, do not push if the
   commit failed, do not open a PR if the push failed, do not amend
   or force-push to work around an error), and do not silently retry.
2. **Surface the exact error to the user** (the command that failed,
   its stderr/stdout, and which step it belongs to).
3. **Wait for the user** to either fix the underlying problem and
   ask you to re-run the skill, or tell you how to proceed.

Per-step instructions below repeat this in places where it is most
easily forgotten, but the rule applies to **every** step, including
ones that do not spell it out explicitly.

## Workflow

Given an `<issue-reference>` (e.g. `123`, `#123`, `myproject#123`, or
`PROJ-123`):

1. **Confirm the issue branch on the server and capture its name.** This
   command is a no-op when the branch already exists; in either case it
   prints the branch name to stdout:
   ```bash
   tod issue create-branch <issue-reference>
   ```
   Save the output as `<issue-branch>`.

2. **Verify the local checkout is on the issue branch.**
   ```bash
   git symbolic-ref --short HEAD
   ```
   If the output does **not** equal `<issue-branch>`, stop and tell the
   user that the working copy is on a different branch and that they
   need to switch to `<issue-branch>` (or run the `work-on-issue` skill)
   before submitting.

3. **Commit any pending work.** Check whether the working copy is clean:
   ```bash
   git status --porcelain
   ```
   - **Empty output** → working copy is clean, skip to step 4.
   - **Non-empty output** → there are uncommitted changes. Apply the
     [`generate-commit-message`](../generate-commit-message/SKILL.md)
     skill **with pull request context** — use the same
     `--source-branch`, `--target-branch`, `--source-project`, and
     `--target-project` values (or omissions) planned for `tod pr create`
     in step 5. Show the proposed message to the user and only after
     explicit confirmation run:
     ```bash
     git add -A
     git commit -m "<subject>" -m "<body>"
     ```
     (Use a `HEREDOC`-style invocation if the message contains multiple
     paragraphs or special characters.) After the commit, re-run
     `git status --porcelain` to confirm the working copy is now clean.

4. **Set up remote tracking and push.**

   a. Identify the OneDev remote that points at this project:
      ```bash
      tod remote
      ```
      Save the output as `<remote>`.

   b. Make sure the local issue branch tracks the same-named branch on
      the server. This is idempotent:
      ```bash
      git fetch <remote> <issue-branch>
      git branch --set-upstream-to=<remote>/<issue-branch> <issue-branch>
      ```

   c. **Capture the commits to be pushed before pushing**, because after
      the push the local branch and its upstream are identical. These
      are the commits that will form the basis of the PR title and
      description in step 5:
      ```bash
      git log --reverse --pretty=format:'%h %s%n%b%n---' <remote>/<issue-branch>..HEAD
      ```
      **If the output is empty**, the issue branch has no new commits
      beyond what is already on the server — there is nothing to
      submit. Stop and tell the user there are no new commits to push.

   d. Push the issue branch:
      ```bash
      git push <remote> <issue-branch>
      ```

5. **Open or attach to the pull request.** First check whether an open pull
   request already includes this issue:
   ```bash
   tod pr list --query 'includes issue "<issue-reference>" and open'
   ```
   Use the same reference form the user gave (e.g. `#123` or `PROJ-123`) inside
   the quotes.

   - **Exactly one match:** Do **not** run `tod pr create`. The push in step 4
     already updated that PR. Surface its reference and URL from the query
     result to the user, then continue to step 6.
   - **Multiple matches:** Stop and ask the user which open PR to use. Do not
     create another pull request until this is resolved.
   - **Zero matches:** Create a new pull request as below.

   When creating a new pull request, decide the `tod pr create` flags first —
   by default omit them all (`--source-branch` defaults to the current
   git branch, which is `<issue-branch>`; `--target-branch` defaults to
   the target project's default branch; `--source-project` and
   `--target-project` default to the current project). Only pass
   `--target-branch`, `--source-project`, `--target-project`, or
   `--merge-strategy` when the user asked for a non-default value.

   a. **Read the title and description requirement** using the same
      flag values (or omissions) planned for `tod pr create`:
      ```bash
      tod pr get-title-and-description-requirement
      ```
      Add any non-default flags from above, for example:
      `--target-branch=<target-branch>`
      `--merge-strategy=<merge-strategy>`. Empty output means no
      requirement.

   b. **Build the PR** from the commits captured in step 4c:

      - **Title**: a concise imperative phrase that summarizes the
        dominant change across those commits.
      - **Description**: a short paragraph explaining what the PR does
        and why.

   c. **Validate** the title and description against any non-empty
      requirement from step 5a.

   Show the proposed title and description to the user and ask for
   confirmation. Only after confirmation, run:
   ```bash
   tod pr create "<title>" --description "<description>"
   ```
   Pass the same non-default flags decided above.

   Surface the server response (which contains the new PR's reference
   and URL) to the user.

6. **Apply deferred OneDev state changes.** When `work-on-issue` drafted
   comments or other state changes that depend on the submitted code, apply
   them **now**, after the push and pull request creation or update succeeded.
   Follow the consent rules in [`using-tod`](../using-tod/SKILL.md).

   Typical examples include posting "Done the work", explaining an
   implemented fix, or making another update whose truth depends on the code
   being available on the server. If there are no deferred changes for this
   session, skip this step. If the push or pull request step failed, do not
   apply them.

7. **Return to the previous branch and remove the local issue branch.** Only
   after steps 5 and 6 succeed:

   a. Determine whether a previous branch exists:
      ```bash
      git rev-parse --abbrev-ref @{-1} 2>/dev/null
      ```
      Save non-empty output as `<previous-branch>`. If the command fails or
      produces empty output, skip the rest of this step — leave the checkout
      on `<issue-branch>`.

   b. If `<previous-branch>` equals `<issue-branch>`, skip the rest of this
      step — there is nowhere else to switch to.

   c. Switch back and delete the local issue branch. The remote branch on
      `<remote>` stays for the open pull request:
      ```bash
      git checkout <previous-branch>
      git branch -D <issue-branch>
      ```

   Tell the user the checkout is now on `<previous-branch>` and the local
   `<issue-branch>` was removed.
