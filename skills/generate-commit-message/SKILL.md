---
name: generate-commit-message
description: Compose a Git commit message that satisfies applicable OneDev branch and pull request requirements. Use when the user asks to draft or generate a commit message. Do not use when running work-on-pull-request, submit-issue-work, or submit-pull-request-work — those workflows compose commit messages themselves.
---

# Generate a OneDev commit message

Compose a commit message for the current change using requirements reported by
OneDev.

## Prerequisites

- `tod` is installed and configured.
- The current OneDev repository contains staged or unstaged changes to
  describe.

## Pull request context

When the commit belongs to an existing or planned pull request, fetch both the
branch and pull request requirements. The message must satisfy every non-empty
requirement.

## Workflow

1. **Read the commit-message requirements.**

   Get the current branch requirement:
      ```bash
      tod get-commit-message-requirement
      ```

   In pull request context, also get the PR requirement:
      ```bash
      tod pr get-commit-message-requirement --target-project <target-project> --target-branch <target-branch> 
      ```
   For an existing PR, use values from its detail. For a planned PR, use the
   values or omissions planned for `tod pr create`.

   Empty output from either requirement command means no requirement from
   that source. When both are empty, fall back to a sensible default
   (short imperative subject, blank line, then body).

2. **Inspect the change.** Use git to see exactly what will be committed
   so the message accurately describes it:
   ```bash
   git status
   git diff --cached         # staged changes
   git diff                  # unstaged changes (only if nothing is staged)
   ```

3. **Build the subject.** Derive a concise imperative subject from the diff
   in step 2. Focus on the dominant change, not every touched file, and
   mention the affected component when it makes the change easier to scan
   in `git log`. Keep the subject short (~50 chars) unless a requirement
   from step 1 says otherwise.

4. **Write the body.** Add a blank line after the subject, then explain
   **why** the change was made and any non-obvious trade-offs, impact, or
   follow-ups. Wrap lines at ~72 chars. Skip the body only for trivial
   commits where the subject is fully self-describing.

5. **Add a footer when relevant.** Common entries:
   - Breaking-change notice, co-authors, or anything else mandated by any
     requirement from step 1.

6. **Validate against the requirement(s).** Re-read every non-empty
   requirement from step 1 and confirm the subject, body, and footer all
   comply with each one.

7. **Present the result.** Show the full proposed message. Only run
   `git commit` when the user also asked to commit and explicitly confirms
   the message.
