---
name: work-on-pull-request
description: Work on a OneDev pull request. Use when the user asks to review, merge, resolve merge conflicts, approve, request changes, address feedback, investigate or fix a failed build, improve, or continue PR work.
---

# Work on a OneDev pull request

Prepare the correct checkout from the user's request, read the relevant context, and perform the work.

## Prerequisites

- `tod` is installed and configured.
- The current repository belongs to the PR's source or target project.

## Session handoff

This workflow pairs with `submit-pull-request-work`. Hand off work within the
**same chat session** using two channels:

| Work product | Handoff |
|--------------|---------|
| Code changes, including merge-conflict resolutions | Local commits on the PR source branch with a clean working copy |
| PR discussion, review, and merge actions | Ordered list held in session as `<saved-pr-actions>` |

When you draft PR comments, code-comment replies, resolve/unresolve actions,
approve/request-changes outcomes, or merge actions, treat each one as **saved
for later submission** -- not merely presentation text. Keep the exact wording
and parameters (file, line range, `comment-id`, commit message, etc.) you
intend to apply. A later `submit-pull-request-work` run in this session must be
able to retrieve `<saved-pr-actions>` and apply them in its step 6.

Do not run any `tod` command that changes PR discussion, review state, or merge
state in this workflow.

## Commit message composition

Compose commit messages in this workflow; do not use the
`generate-commit-message` skill. Empty requirement output means no requirement
from that source.

- For local code-change commits, read:
  ```bash
  tod get-commit-message-requirement
  tod pr get-commit-message-requirement --target-project <target-project> --target-branch <target-branch>
  ```
  Inspect the diff, satisfy every non-empty requirement, and ask the user to
  confirm the full message before running `git commit`.
- For squash-merge actions, read:
  ```bash
  tod pr get-commit-message-requirement --target-project <target-project> --target-branch <target-branch>
  tod issue list --query 'fixed in pull request "<pr-reference>"'
  ```
  If `<fixed-issues>` was already saved while determining the work
  specification, reuse it instead of running the fixed-issue query again.
  Compose the message from the PR title, description, and fixed issues,
  satisfy every non-empty requirement, and save the full message in
  `<saved-pr-actions>`.

## Stop on error

Run the workflow sequentially. On any command failure, missing required
output, failed precondition, or declined confirmation, stop immediately,
report the command and error, and wait for the user. Do not continue, repair
state beyond the current step, or retry silently.

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

2. **Prepare the checkout.** Prepare the checkout to work on the PR, unless the user
   explicitly asks not to, or wants to switch to a different checkout.

   Check out the PR:
   ```bash
   tod pr checkout <pr-reference>
   ```
   The command validates that the current project is either the PR source
   project or target project, and fails if the working directory has
   uncommitted changes.

3. **Determine the work specification.** The work may come from the user's prompt directly,
    from someone's feedback on the PR, from the PR itself (title and
    description), or from the descriptions and comments of issues fixed by the PR.

   Even when the prompt is the primary specification, do not proceed from the
   prompt alone. The PR title, description, associated comments, code comments,
   and fixed issue context are important context, and can be retrieved as below:

   | Context source | How to inspect |
   |----------------|----------------|
   | Fixed issues | `tod issue list --query 'fixed in pull request "<pr-reference>"'` -- save the output as `<fixed-issues>`. For each returned issue, inspect `tod issue get <issue-reference>` and `tod issue get-comments <issue-reference>`; note the issue title, description, and comments. |
   | PR metadata, title, and description | `tod pr get <pr-reference>` -- note `<source-project>`, `<source-branch>`, `<target-project>`, `<target-branch>`, `<merge-strategy>`, `<head-commit>`, `<status>`, title, description, linked issues, submitter, reviewers, assignees, and current review status. |
   | PR comments | `tod pr get-comments <pr-reference>` |
   | Line-anchored code comments | `tod pr get-code-comments <pr-reference>` -- note `id`, file, line range, resolution state, and replies. |

   Reuse `<fixed-issues>` later if squash-merge commit message composition
   needs the same fixed-issue query result.

   Then proceed to get your own login name:
   ```bash
   tod get-login-name
   ```

   Match your login name against the PR submitter, reviewers, assignees, and author of each PR comment,
   code comment, and reply to understand your role, position, and previous involvements in
   the context.

   When the work is to investigate or fix a failed build, treat the relevant
   `<build-reference>` as required context for assessment.

   Read the current patch to understand what the PR already changes relative
   to its target:
   ```bash
   tod pr get-patch <pr-reference>
   ```
   When the prompt asks only to review or respond, use the review-scoped patch
   instead:
   ```bash
   tod pr get-patch <pr-reference> --for-code-review
   ```
   When the patch alone is insufficient, inspect the corresponding files in
   the checked-out working copy.

   **Inspect embedded resources.** Download every linked image or file from
   the PR description, PR discussion, and fixed issue descriptions/comments:

   - Find image and file links (`![alt](url)` and `[label](url)`) in every
     text source you read above.
   - For each URL, save it with the URL **exactly** as written:
     ```bash
     tod download <resource-url> <output-file>
     ```
   - Open images and read other files as needed.

4. **Assess, plan, and execute the work.** Check the requested work against the current
   code and behavior, plan appropriate actions, and execute it.

   If the work is to review the PR, form the review following below procedure:

   - Read/search file contents as needed in the checked-out PR head.
   - For a general review, check correctness, edge cases, security, style,
     and test coverage.
   - Prefer line-anchored code comments for findings on the right side of the
     patch. The line range must be visible as an added line (`+`) or unchanged
     context line in the current patch. Do not attach a finding to an unrelated
     patch line merely to create a code comment.
   - Put outside-patch findings in the overall PR comment with file paths and
     line numbers when available.
   - Triage your prior discussion. For your prior general PR comments,
     include any needed follow-up in the new overall PR comment. For prior
     code comments, resolve addressed concerns, reply when new information is
     needed, unresolve resurfaced concerns, and leave unchanged unresolved
     concerns alone. Do not modify another user's code comments unless the
     user explicitly asks.
   - If the login name is a pending reviewer in the PR metadata, plan
     `tod pr approve <pr-reference>` or `tod pr request-changes <pr-reference>`
     as a review outcome; otherwise plan `tod pr add-comment` as a review
     summary.

   If the work is to merge the PR:

   - If `<merge-strategy>` is `SQUASH_SOURCE_BRANCH_COMMITS`, fetch the PR commit
     message using **Commit message composition**.
   - Save a merge action for later submission: `tod pr merge <pr-reference>`
     when no commit message is needed, or `tod pr merge <pr-reference>
     --commit-message '<commit-message>'` for squash merges.

   If the work is to resolve merge conflicts:

   - Fetch the target branch head of `<target-project>`:
     ```bash
     git fetch <target-project-remote-or-url> <target-branch>
     git rev-parse FETCH_HEAD
     ```
     Save `FETCH_HEAD` as `<target-head>`.
   - Merge `<target-head>` into the current checkout with the saved
     `<merge-strategy>`, but do not commit or push. For example, use
     `git merge --no-commit <target-head>` for merge-commit strategies and
     `git merge --squash <target-head>` for `SQUASH_SOURCE_BRANCH_COMMITS`.
     If the strategy is `REBASE_SOURCE_BRANCH_COMMITS`, stop and ask for user
     direction before rewriting commits.
   - If there are conflicts, resolve them in the current checkout. Then run:
     ```bash
     git status --porcelain
     ```
     Confirm the output has no unmerged entries such as `UU`, `AA`, `DD`, `AU`,
     `UA`, `DU`, or `UD`.

   If the work is to investigate or fix a failed build for a specified
   `<build-reference>`:

   - Gather and examine build evidence before planning code changes:
     ```bash
     tod build get <build-reference>
     tod build get-log <build-reference>
     ```
   - Read the build detail and log content carefully to identify the failure.
     Note the build `commitHash`. The failure happened on that commit, while
     fixes must be implemented in the current checked-out PR head; those
     commits may differ. Use the build commit as failure evidence, and verify
     whether each candidate fix still applies to the current checkout before
     editing.
   - If the log contains a statement like
     `Dependency build is required to be successful but failed: <dependency-build-reference>`,
     get the dependency build detail. If its commit hash is the same as the
     current build, investigate or fix the dependency build failure instead;
     repeat this process for same-commit dependency build failures. If the
     dependency build's commit hash differs from the current build, conclude
     that the current build failure is caused by this dependency build.
   - If the log contains a statement like
     `<report-name>: found problems with severity <severity-level> or higher`,
     fetch the referenced problems report:
     ```bash
     tod build get-code-problems <build-reference> <report-name> <severity-level>
     ```
     Problems may point to workspace files, 1-based line ranges, or
     non-workspace artifacts used by the project.
   - Inspect referenced workspace files as necessary. Inspect
     `.onedev-buildspec.yml` when job configuration may be involved, and
     run below command to get its schema if you need to modify it:
     ```bash
     tod build get-spec-schema
     ```
   - If useful, inspect changes since the previous successful build:
     ```bash
     tod build get-changes-since-success <build-reference>
     ```
   - Implement any needed code changes in the current checkout. If no code
     change is appropriate, draft the PR comment or review action explaining
     the decision.

   Do not run any `tod` command that changes PR discussion, review state, or
   merge state during this workflow. After implementing a code change, or after
   deciding that the right outcome is only a response or state change, draft
   every planned PR comment, code-comment reply, resolve, unresolve, approval,
   request-changes action, merge action, or other PR state change.

   If code changes remain in the working copy, commit them locally before
   presenting the result:
   ```bash
   git status --porcelain
   ```
   If the output is non-empty, compose the local commit message using
   **Commit message composition**, then after user confirmation run:
   ```bash
   git add -A
   git commit -m '<subject>' -m '<body>'
   git status --porcelain
   ```
   The final status must be clean.

   Save each planned action in `<saved-pr-actions>` (see **Session handoff**).
   Preserve action type and parameters needed by `submit-pull-request-work`
   step 6, for example:
   - New line-anchored finding -- file, line range, comment text
   - General PR feedback -- comment text
   - Line-anchored thread -- `comment-id`, reply text
   - Resolve or unresolve -- `comment-id`, note text
   - Reviewer outcome -- approve or request-changes, with summary text when
     applicable
   - Merge outcome -- whether it needs `--commit-message`, and the full message
     when applicable

   Draft all comment, reply, note, and review text in Markdown, as the
   corresponding `tod` commands post Markdown text.

   When someone should be mentioned in comments or notes, mention each user
   only once across all drafted comments and notes for this submission -- not
   in every draft.

   When code was changed, leave the working copy on the PR source branch with
   the new local commits. For review-only or response-only work, leave the
   checkout as prepared for inspection. In all cases, keep work ready for
   `submit-pull-request-work`.
