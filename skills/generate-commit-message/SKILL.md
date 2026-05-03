---
name: generate-commit-message
description: Generate a git commit message for the current change in a OneDev
  project that conforms to the project's commit-message requirement, derives
  the subject from the active issue when one is set (and from the diff
  otherwise), and includes a meaningful body or footer. Use when the user
  asks to write, draft, generate, or compose a commit message.
---

# Generate a OneDev commit message

This skill walks the agent through composing a git commit message for the
current change, using the `tod` CLI to discover the project's commit-message
requirement and any active issue reference.

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config set` if needed).
- The current working directory is inside a git repository pointing at a
  OneDev project, with the change you want to commit either staged or
  present in the working tree.

## Workflow

1. **Read the commit-message requirement.** Fetch the project's rule for
   non-merge commit messages so the generated message conforms to it:
   ```bash
   tod get-commit-message-requirement
   ```
   Empty output means the project has no requirement; fall back to a
   sensible default (short imperative subject, blank line, then body).

2. **Inspect the change.** Use git to see exactly what will be committed
   so the message accurately describes it:
   ```bash
   git status
   git diff --cached         # staged changes
   git diff                  # unstaged changes (only if nothing is staged)
   ```

3. **Build the subject.** Try the current issue first, then fall back to
   the diff:
   - Run `tod issue current-reference`. If it prints issue reference, fetch the
     issue and use its `title` as the basis of the subject:
     ```bash
     tod issue get <ref>
     ```
     Trim the title to a concise imperative phrase, rephrasing only when
     needed for clarity or length.
   - If the reference is empty, derive a concise imperative subject
     directly from the diff. Focus on the dominant change, not every
     touched file, and mention the affected component when it makes the
     change easier to scan in `git log`.

   Keep the subject short (~50 chars) unless the requirement from step 1
   says otherwise.

4. **Write the body.** Add a blank line after the subject, then explain
   **why** the change was made and any non-obvious trade-offs, impact, or
   follow-ups. Wrap lines at ~72 chars. Skip the body only for trivial
   commits where the subject is fully self-describing.

5. **Add a footer when relevant.** Common entries:
   - Breaking-change notice, co-authors, or anything else mandated by the
     requirement from step 1.

6. **Validate against the requirement.** Re-read the requirement from
   step 1 and confirm the subject, body, and footer all comply.

7. **Show the message and ask before committing.** Print the full proposed
   message to the user. Only run `git commit` after explicit confirmation.
