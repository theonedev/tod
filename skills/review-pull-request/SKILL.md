---
name: review-pull-request
description: Review a OneDev pull request and provide actionable feedback. Use when the user asks to review, approve, request changes on, or comment on a OneDev pull request.
---

# Review a OneDev pull request

This skill walks the agent through a complete pull request review using the
`tod pr` and `tod code-comment` subcommands. It covers reading context,
temporarily checking out the PR head, following review instructions from the
user's prompt, leaving line-anchored code comments on problematic snippets,
reconciling the agent's own prior code comments against the current patch,
and finalizing the review.

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config set` if needed).
- The current working directory is inside a git repository pointing at the
  OneDev project that owns the pull request (or the PR uses a reference that
  includes the project, e.g. `42`, `myproject#42`).

## Stop on error

**The steps below are sequential and each one depends on the previous
one succeeding.** If any command in the workflow fails — non-zero exit
code, an error message on stderr, empty output where output is
required, or a precondition check (e.g. clean working directory) that
does not hold — you **must**:

1. **Immediately stop the workflow.** Do not run any later step, do
   not "try the next thing anyway", do not attempt to repair state
   beyond what the step itself describes, and do not silently retry.
2. **Surface the exact error to the user** (the command that failed,
   its stderr/stdout, and which step it belongs to).
3. **Wait for the user** to either fix the underlying problem and
   ask you to re-run the skill, or tell you how to proceed.

Per-step instructions below repeat this in places where it is most
easily forgotten, but the rule applies to **every** step, including
ones that do not spell it out explicitly.

## Workflow

Given a `<pr-reference>` (e.g. `42`, `#42`, `myproject#42`, or `PROJ-42`):

1. **Determine the review specification.** The review instructions may come
   from the user's prompt directly or from the PR context:

   | Review instruction source | Primary specification | PR context |
   |---------------------------|-----------------------|------------|
   | User prompt specifies review instructions | The prompt (scope, focus areas, criteria, constraints, or requested outcome beyond naming the PR) | Fetch below; use PR metadata and discussion as supplementary context |
   | Prompt only names the PR or asks for a general review | The complete review workflow in this skill | PR metadata and discussion provide the review context |

   Follow concrete review instructions from the prompt directly. Still
   complete the context-gathering steps below; prompt instructions do not
   justify skipping the PR metadata, patch, discussion, or relevant
   attachments.

2. **Read the PR detail.** Get the title, description, source/target branches,
   head commit hash, reviewers, current review status, and any linked issues:
   ```bash
   tod pr get <pr-reference>
   ```
   Save `headCommitHash` as `<head-commit>`.

3. **Fetch and check out the PR head commit.** This is a temporary detached
   checkout used only for the review.

   a. **Require a clean working directory.**
      ```bash
      git status --porcelain
      ```
      Any output means there are uncommitted changes; stop and ask the user
      to commit or stash them before re-running the skill.

   b. **Identify the OneDev remote, fetch the head commit, and check it out
      without moving or creating a local branch.**
      ```bash
      tod remote
      git fetch <remote> <head-commit>
      git checkout --detach <head-commit>
      ```

   c. **Verify the checkout.**
      ```bash
      git rev-parse HEAD
      ```
      The output must equal `<head-commit>`. Otherwise stop.

4. **Read the review-scoped file changes.** `--for-code-review` filters out
   files excluded by the project's AI settings so the diff stays focused:
   ```bash
   tod pr get-patch <pr-reference> --for-code-review
   ```
5. **Read existing discussion.** Pull both general and code-anchored
   comments so the review acknowledges prior context and you know which
   code comments you previously authored:
   ```bash
   tod pr get-comments <pr-reference>
   tod pr get-code-comments <pr-reference>
   tod get-login-name
   ```
   Use the login name to distinguish your own prior discussion from feedback
   written by other users:

   - In the output of `tod pr get-comments`, a general PR comment whose
     `user` matches the login name is your own prior comment. Treat it as
     prior review context. If it needs correction or follow-up, include that
     update in the new overall PR comment; general comments do not have a
     reply or resolve operation.
   - In the output of `tod pr get-code-comments`, a code comment whose
     `user` matches the login name is your own prior code comment. These are
     the comments you may reply to, resolve, or unresolve during this review.
   - Treat comments from other users as collaborator feedback to consider,
     but do not modify their code-comment state unless the user explicitly
     asks you to.

   **Download and inspect embedded resources.** The PR description (step 2),
   general comments, line-anchored code comments, and replies on those code
   comments (all from steps 2 and 5) are often markdown with screenshots,
   diagrams, or other files. Text alone is not enough when links are
   present — you must download and check each attachment:

   - Find image and file links in the description, every general comment,
     every code comment body, and every reply (`![alt](url)` and
     `[label](url)`). Scan the JSON from `tod pr get-code-comments` for
     comment content and nested replies.
   - For each URL, save it locally using the URL **exactly** as it
     appears in the markdown (do not rewrite or normalize it):
     ```bash
     tod download <resource-url> <output-file>
     ```
   - Open images with the Read tool; read other downloaded files as
     needed. Do not skip this when attachments are linked — they often
     document expected behavior, UI, or failures relevant to the review.
6. **Read/search file contents as needed.** For file contents whose context
   matters beyond the hunk shown in the diff, read and search them in the
   checked-out PR head.
7. **Form the review.** Apply the review specification from step 1. For a
   general review, check correctness, edge cases, security, style, and test
   coverage. For each finding, prefer a **line-anchored code comment** tied
   to a specific file and line range over a paragraph in the summary. Plan
   four buckets before posting anything:
   - **New code comments** for problematic snippets in the current patch.
     Each must reference a line range visible on the **right side** of the
     patch — added lines (`+`) or unchanged context lines. Lines that exist
     only on the old (left) side cannot be commented on.
   - **Outside-patch findings** for concerns discovered in code that is not
     visible on the right side of the patch. Put these directly in the
     overall PR comment, including the relevant file path and line number
     when available. Do not attach them to an unrelated patch line merely
     to create a code comment.
   - **Triage of your prior discussion** (comments whose `user` matches
     `tod get-login-name`) against the new patch:
     - For a prior general PR comment that needs clarification, correction,
       or follow-up → include the update in the new overall PR comment.
     - For each prior code comment:
       - The concern is now addressed by the patch → resolve.
       - New information would clarify or extend the concern → reply.
       - A previously resolved concern has resurfaced → unresolve.
       - An unresolved concern is unchanged → take no action.
   - **Overall outcome and actions:**
     - Post patch findings with `tod pr add-code-comment`.
     - Update your prior code comments with `tod code-comment add-reply`,
       `resolve`, or `unresolve` as planned.
     - Include outside-patch findings and follow-ups to your prior general
       PR comments in the overall review comment.
     - If the login name from step 5 is a pending reviewer in step 2, choose
       `tod pr approve` or `tod pr request-changes`. Otherwise leave a
       general comment via `tod pr add-comment`.
8. **Request the user's consent** before posting anything. Summarize the
   planned new code comments, outside-patch findings for the overall PR
   comment, follow-ups to your prior general PR comments, code-comment
   replies/resolves/unresolves, and overall outcome, then wait for explicit
   approval.
9. **Execute the approved review.** Post only the actions and overall
   outcome approved in step 8.
10. **Restore the previous checkout.** Do this only after the approved review
    actions have completed successfully.

    a. Determine whether a previous branch exists:
       ```bash
       git rev-parse --abbrev-ref @{-1} 2>/dev/null
       ```
       Save non-empty output as `<previous-branch>`. If the command fails or
       produces empty output, skip the rest of this step and leave the
       checkout on the PR head.

    b. Switch back to the previous branch:
       ```bash
       git checkout <previous-branch>
       ```
       Do not delete branches or discard files as part of restoration.
