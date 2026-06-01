---
name: generate-commit-message
description: Generate a git commit message for the current change in a OneDev
  project that conforms to the project's commit-message requirement (and, when
  the commit will be part of a pull request, the pull request commit-message
  requirement) and includes a meaningful body or footer. Use when the user
  asks to write, draft, generate, or compose a commit message.
---

# Generate a OneDev commit message

This skill walks the agent through composing a git commit message for the
current change, using the `tod` CLI to discover the project's commit-message
requirement.

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config set` if needed).
- The current working directory is inside a git repository pointing at a
  OneDev project, with the change you want to commit either staged or
  present in the working tree.

## Pull request context

Use pull request context whenever the commit will be included in a pull
request — whether the PR already exists or is about to be created.

In step 1 below, fetch **both** the branch requirement and the pull
request requirement; the composed message must satisfy every non-empty
requirement.

## Workflow

1. **Read the commit-message requirement(s):

   a. Get message requirement for the current branch:
      ```bash
      tod get-commit-message-requirement
      ```

   b. When pull request context applies, also fetch the pull request commit
      message requirement:
      ```bash
      tod pr get-commit-message-requirement --target-project=<target project> --target-branch=<target branch> --source-project=<source project> --source-branch=<source branch>
      ```
      For existing PR, resolve above params from PR detail; for planned PR, use same 
      value (or omissions) planned for `tod pr create`

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

7. **Show the message and ask before committing.** Print the full proposed
   message to the user. Only run `git commit` after explicit confirmation.
