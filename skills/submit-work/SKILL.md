---
name: submit-work
description: Submit completed work on a OneDev issue via the `tod` CLI — confirm
  the issue branch exists on the server, verify the local checkout is on it,
  commit any pending changes (using the `generate-commit-message` skill), push
  to the matching remote branch, and open a pull request whose title and
  description summarize the commits added on the issue branch. Use when the
  user asks to submit, complete, or finish work on a OneDev issue (e.g.
  "submit work #123", "complete work on PROJ-7", "finish issue myproject#42").
---

# Submit work on a OneDev issue

This skill takes a OneDev issue from "implementation done in the working
copy" to "pull request opened on the server". It assumes the agent has
already implemented the change on the issue branch (typically after
running the `work-on-issue` skill).

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

Given an `<issue-reference>` (e.g. `#123`, `myproject#123`, or
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
     skill to compose a commit message, show it to the user, and only
     after explicit confirmation run:
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

5. **Create the pull request.** Build the PR from the commits captured
   in step 4c:

   - **Title**: a concise imperative phrase that summarizes the dominant
     change across those commits. If there is exactly one commit, reuse
     its subject. Otherwise synthesize a single subject that captures
     the overall intent (the issue title from `tod issue get
     <issue-reference>` is a useful sanity-check reference). **Do not
     include the issue reference in the title** — the source branch
     already links the PR to the issue, so repeating it adds noise.
     **If the user indicated that the work is still in progress** (e.g.
     they said "submit WIP", "work in progress", "draft", or otherwise
     made clear the change is not ready for merge), prefix the title
     with `[WIP] ` (for example: `[WIP] Refactor build cache eviction`).
     Otherwise, do not add the prefix.
   - **Description**: a short paragraph explaining what the PR does and
     why, followed by a bulleted list of the commits (one bullet per
     commit, using each commit's subject; expand with body context when
     it adds non-obvious information).

   Show the proposed title and description to the user and ask for
   confirmation. Only after confirmation, run:
   ```bash
   tod pr create "<title>" --description "<description>"
   ```
   (`--source-branch` defaults to the current git branch, which is
   `<issue-branch>`; `--target-branch` defaults to the project's default
   branch, which is normally what you want.)

   Surface the server response (which contains the new PR's reference
   and URL) to the user.
