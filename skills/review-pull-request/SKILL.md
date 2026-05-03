---
name: review-pull-request
description: Perform a structured code review of a OneDev pull request via the `tod` CLI, examining metadata, file changes, and referenced files, then approving, requesting changes, or leaving a comment. Use when the user asks to review, approve, request changes on, or comment on a OneDev pull request.
---

# Review a OneDev pull request

This skill walks the agent through a complete pull request review using the
`tod pr` subcommands.

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config init` if needed).
- The current working directory is inside a git repository pointing at the
  OneDev project that owns the pull request (or the PR uses a reference that
  includes the project, e.g. `myproject#42`).

## Workflow

Given a `<pr-reference>` (e.g. `#42`, `myproject#42`, or `PROJ-42`):

1. **Read the PR metadata.** Get title, description, source/target branches,
   reviewers, current review status, and any linked issues:
   ```bash
   tod pr show <pr-reference>
   ```
2. **Read the review-scoped file changes.** `--for-code-review` filters out
   files excluded by the project's AI settings so the diff stays focused:
   ```bash
   tod pr file-changes <pr-reference> --for-code-review
   ```
3. **Read full file contents as needed.** For files whose context matters
   beyond the hunk shown in the diff, fetch the full content at either side
   of the PR:
   ```bash
   tod pr file-content <pr-reference> --path <path>               # after change
   tod pr file-content <pr-reference> --path <path> --old-revision # before change
   ```
4. **Form the review.** Check correctness, edge cases, security, style, and
   test coverage. Collect concrete feedback tied to specific files and line
   ranges.
5. **Decide the outcome and request the user's consent** before posting. Find
   out if you are a pending reviewer:
   ```bash
   tod whoami
   ```
   Compare the login name with the reviewers in the PR metadata from step 1.
6. **Post the review:**

   - If you are a pending reviewer and the user has consented, use
     `tod pr process` with `approve` or `requestChanges`:
     ```bash
     tod pr process <pr-reference> \
       --operation approve \
       --comment "<summary>"
     ```
     ```bash
     tod pr process <pr-reference> \
       --operation requestChanges \
       --comment "<summary>"
     ```
   - Otherwise (or when the user prefers a general comment), use
     `tod pr comment`:
     ```bash
     tod pr comment <pr-reference> --content "<review body>"
     ```

## Tips

- Always gate `tod pr process` behind explicit user consent — it is a
  state-changing operation.
- The `--operation` argument also accepts `merge`, `discard`, `reopen`,
  `deleteSourceBranch`, and `restoreSourceBranch` when the user asks for
  one of those actions after the review.
- Use `tod pr code-comments <ref>` and `tod pr comments <ref>` if prior
  reviews left context you should acknowledge.
