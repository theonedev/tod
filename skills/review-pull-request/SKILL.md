---
name: review-pull-request
description: Perform a structured code review of a OneDev pull request via the `tod` CLI, examining metadata, patch, referenced files, and prior code comments, then leaving line-anchored code comments, replies, resolutions, and an overall approve/request-changes/comment decision. Use when the user asks to review, approve, request changes on, or comment on a OneDev pull request.
---

# Review a OneDev pull request

This skill walks the agent through a complete pull request review using the
`tod pr` and `tod code-comment` subcommands. It covers reading context,
leaving line-anchored code comments on problematic snippets, reconciling
the agent's own prior code comments against the current patch, and
finalizing the review.

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config set` if needed).
- The current working directory is inside a git repository pointing at the
  OneDev project that owns the pull request (or the PR uses a reference that
  includes the project, e.g. `myproject#42`).

## Workflow

Given a `<pr-reference>` (e.g. `#42`, `myproject#42`, or `PROJ-42`):

1. **Read the PR metadata.** Get title, description, source/target branches,
   reviewers, current review status, and any linked issues:
   ```bash
   tod pr get <pr-reference>
   ```
2. **Read the review-scoped file changes.** `--for-code-review` filters out
   files excluded by the project's AI settings so the diff stays focused:
   ```bash
   tod pr get-patch <pr-reference> --for-code-review
   ```
3. **Read existing discussion.** Pull both general and code-anchored
   comments so the review acknowledges prior context and you know which
   code comments you previously authored:
   ```bash
   tod pr get-comments <pr-reference>
   tod pr get-code-comments <pr-reference>
   tod get-login-name
   ```
   Match the login name against the `user` field on each code comment to
   find your own prior comments — those are the ones you may later reply
   to, resolve, or unresolve.

   **Download and inspect embedded resources.** The PR description (step 1),
   general comments, line-anchored code comments, and replies on those code
   comments (all from steps 1 and 3) are often markdown with screenshots,
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
4. **Read full file contents as needed.** For files whose context matters
   beyond the hunk shown in the diff, fetch the full content at either side
   of the PR:
   ```bash
   tod pr get-file-content <pr-reference> <path>                # after change
   tod pr get-file-content <pr-reference> <path> --old-revision # before change
   ```
5. **Form the review.** Check correctness, edge cases, security, style, and
   test coverage. For each finding, prefer a **line-anchored code comment**
   tied to a specific file and line range over a paragraph in the summary.
   Plan three buckets before posting anything:
   - **New code comments** for problematic snippets in the current patch.
     Each must reference a line range visible on the **right side** of the
     patch — added lines (`+`) or unchanged context lines. Lines that exist
     only on the old (left) side cannot be commented on.
   - **Triage of your prior code comments** (those whose `user` matches
     `tod get-login-name`) against the new patch:
     - The concern is now addressed by the patch → resolve.
     - The concern still applies, or the patch needs follow-up → reply.
     - A previously resolved concern has resurfaced → unresolve.
     Leave comments authored by other users alone unless the user asks
     otherwise.
   - **Overall outcome** — approve, request changes, or post a general
     comment. To know whether you can approve or request changes, compare
     the login name from step 3 with the reviewers listed in step 1; only
     pending reviewers can do those operations.
6. **Request the user's consent** before posting anything. Summarize the
   planned new code comments, replies/resolves/unresolves, and overall
   outcome, then wait for explicit approval.
7. **Post line-anchored feedback.**
   - Add new code comments for problematic snippets:
     ```bash
     tod pr add-code-comment <pr-reference> "<comment>" \
         --file <path> --from-line <n> [--to-line <n>]
     ```
     `--to-line` defaults to `--from-line` when omitted. The range must lie
     on the right side of the patch as described in step 5.
   - Reply to or resolve/unresolve your prior code comments using the `id`
     values from `tod pr get-code-comments`:
     ```bash
     tod code-comment add-reply <comment-id> "<reply>"
     tod code-comment resolve   <comment-id> --note "<why it's settled>"
     tod code-comment unresolve <comment-id> --note "<why it's reopened>"
     ```
8. **Post the overall review:**
   - As a pending reviewer with consent, use `approve` or `request-changes`:
     ```bash
     tod pr approve <pr-reference> --comment "<summary>"
     ```
     ```bash
     tod pr request-changes <pr-reference> --comment "<summary>"
     ```
   - Otherwise (or when the user prefers a general comment), use
     `add-comment`:
     ```bash
     tod pr add-comment <pr-reference> "<review body>"
     ```
