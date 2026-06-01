---
name: submit-pull-request-work
description: Submit completed work on a OneDev pull request via the `tod` CLI
  — verify the checkout is on the PR's source branch in the source project,
  commit any pending changes (using the `generate-commit-message` skill),
  push to the matching remote branch so the existing pull request picks up the
  new commits, then post any deferred PR/issue/code-comment replies from
  work-on-pull-request. Use when the user asks to submit, push, or finish work
  on a pull request (e.g. "submit PR work", "push my PR fixes", "submit work
  on PR 42").
---

# Submit pull request work

This skill publishes local changes to an **existing** pull request and, when
applicable, posts OneDev replies that were deferred in
[`work-on-pull-request`](../work-on-pull-request/SKILL.md) until after push.
It assumes the agent has already addressed feedback in the working copy
(typically after running `work-on-pull-request`). It does **not** create a new
pull request.

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config set` if needed).
- The current working directory is inside a git repository whose OneDev
  remote points at the **source project** of the pull request.

## Stop on error

**The steps below are sequential and each one depends on the previous
one succeeding.** If any command in the workflow fails — non-zero exit
code, an error message on stderr, empty output where output is
required, a failed precondition check (e.g. "wrong branch", "no new
commits to push"), or the user declining to confirm a commit message —
you **must**:

1. **Immediately stop the workflow.** Do not run any later step, do
   not "try the next thing anyway" (for example, do not push if the
   commit failed, do not amend or force-push to work around an error),
   and do not silently retry.
2. **Surface the exact error to the user** (the command that failed,
   its stderr/stdout, and which step it belongs to).
3. **Wait for the user** to either fix the underlying problem and
   ask you to re-run the skill, or tell you how to proceed.

Per-step instructions below repeat this in places where it is most
easily forgotten, but the rule applies to **every** step, including
ones that do not spell it out explicitly.

## Workflow

Resolve a `<pr-reference>` (e.g. `42`, `#42`, `myproject#42`, or
`PROJ-42`) from the user's message. When they did not name one, use the
PR from a recent `work-on-pull-request` session. If still unknown, query
for an open PR whose source branch is the current branch:
```bash
git symbolic-ref --short HEAD
tod pr list --query 'open and "Source Branch" is "<current-branch>"'
```
Exactly one match gives the reference; zero or multiple matches mean you
should ask the user.

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

2. **Verify the local checkout is on the source branch.**
   ```bash
   git symbolic-ref --short HEAD
   ```
   If the output does **not** equal `<source-branch>`, stop and tell the
   user to check out the PR (run the `work-on-pull-request` skill) before
   submitting.

3. **Commit any pending work.** Check whether the working copy is clean:
   ```bash
   git status --porcelain
   ```
   - **Empty output** → working copy is clean, skip to step 4.
   - **Non-empty output** → there are uncommitted changes. Apply the
     [`generate-commit-message`](../generate-commit-message/SKILL.md)
     skill **with pull request context** — pass PR reference and detail 
     from step 1. Show the proposed message to the user and only after 
     explicit confirmation run:
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

5. **Confirm the update.** Surface `<commits-to-push>` (when non-empty) and
   the PR reference/URL from step 1 to the user. When you pushed, the existing
   pull request now includes the new commits; do **not** run `tod pr create`.

6. **Post deferred follow-up on OneDev.** When `work-on-pull-request` drafted
   replies or resolves that depend on pushed code, post them **now** — only
   after step 4d succeeded when there was a push, or immediately when
   `<commits-to-push>` was empty but those fixes are already on the server.

   Follow the consent rules in [`using-tod`](../using-tod/SKILL.md). When
   helpful, mention a commit from `<commits-to-push>` (e.g. the latest short
   hash) so threads are easy to verify. Match the channel:
   - General PR feedback → `tod pr add-comment <pr-reference> "<reply>"`
   - Line-anchored thread → `tod code-comment add-reply <comment-id> "<reply>"`
   - Concern addressed in code → `tod code-comment resolve <comment-id> --note "<why>"` when appropriate
   - Issue-side discussion → `tod issue add-comment <issue-ref> "<reply>"` when the thread lives on the issue

   If there are no deferred writes for this session, skip this step. If a push
   was required but step 4d did not run or failed, do **not** post deferred
   replies.
