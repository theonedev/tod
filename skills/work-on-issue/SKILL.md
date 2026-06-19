---
name: work-on-issue
description: Implement work for a OneDev issue. Use when the user asks to start, pick up, or continue issue work.
---

# Work on a OneDev issue

Set up the issue branch, read the relevant context, and implement the work.

## Prerequisites

- `tod` is installed and configured.
- The current repository belongs to the project that owns the issue.

## Stop on error

Run the workflow sequentially. On any command failure, missing required
output, or failed precondition, stop immediately, report the command and
error, and wait for the user. Do not continue, repair state beyond the
current step, or retry silently.

## Workflow

Given an `<issue-reference>` (e.g. `123`, `#123`, `myproject#123`, or
`PROJ-123`):

1. **Create the issue branch on the server** and capture its name. The
   command is a no-op when the branch already exists; in either case it
   prints the branch name to stdout:
   ```bash
   tod issue create-branch <issue-reference>
   ```
   Save the output as `<issue-branch>`.

2. **Switch the local checkout to `<issue-branch>`.** First check the
   current branch:
   ```bash
   git symbolic-ref --short HEAD
   ```
   If it already equals `<issue-branch>`, skip ahead to step 3.
   Otherwise:

   a. **Require a clean working directory.**
      ```bash
      git status --porcelain
      ```
      Any output means there are uncommitted changes; stop and ask the
      user to commit or stash them before re-running the skill.

   b. **Identify the OneDev remote** that points at this project:
      ```bash
      tod remote
      ```
      Save the output as `<remote>`.

   c. **Check whether the local branch already exists:**
      ```bash
      git rev-parse --verify --quiet <issue-branch>
      ```

   d. **Switch to the branch.**
      - If the local branch does **not** exist, fetch it from the server
        and create a local branch from the fetched remote ref:
        ```bash
        git fetch <remote> <issue-branch>
        git checkout -b <issue-branch> <remote>/<issue-branch>
        ```
      - If the local branch **already** exists, just check it out:
        ```bash
        git checkout <issue-branch>
        ```

   e. **Set up remote tracking** in both cases so future `pull`/`push`
      target the issue branch on the server:
      ```bash
      git branch --set-upstream-to=<remote>/<issue-branch> <issue-branch>
      ```

3. **Determine the work specification.** The
   work to do may come from the user's prompt directly or from the issue
   on OneDev:

   | Work instruction source | Primary specification | Issue context |
   |-------------------------|---------------------|---------------|
   | User prompt specifies the work | The prompt (concrete task, scope, approach, or constraints beyond naming the issue) | Fetch below; use title, description, and comments as supplementary background |
   | Prompt only names the issue | Issue title and description | Comments are supplementary (clarifications, constraints, hints) |

   When the prompt is the primary specification, still fetch issue metadata
   and discussion as context — do not skip ahead to step 4 from partial output:
   ```bash
   tod issue get <issue-reference>
   tod issue get-comments <issue-reference>
   tod get-login-name
   ```

   When the prompt only names the issue, the same commands are required;
   without that context you cannot plan the work reliably.

   Save the login name and match it against each comment's author to
   recognize comments you previously wrote. Treat those as your own prior
   context rather than independent collaborator feedback.

   **When the work is to investigate a build failure or fix a failed build,
   gather and examine build evidence before planning code changes.** Given
   the relevant `<build-reference>`:
   ```bash
   tod build get <build-reference>
   tod build get-log <build-reference>
   ```
   Read the build detail and log content carefully to identify the failure:

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
     ```
     tod build get-spec-schema
     ```
   - If useful, inspect changes since the previous successful build:
     ```bash
     tod build get-changes-since-success <build-reference>
     ```

   **Inspect embedded resources.** Download every linked image or file from
   the issue description and comments:

   - Find image and file links in the description and every comment
     (`![alt](url)` and `[label](url)`).
   - For each URL, save it locally using the URL **exactly** as it
     appears in the markdown (do not rewrite or normalize it):
     ```bash
     tod download <resource-url> <output-file>
     ```
   - Open images and read other downloaded files as needed.

4. **Assess, plan, and execute.** Check the requested work against the
   current code and behavior before deciding that a code change is needed.

   - If the request is reasonable and not already implemented, summarize
     what you will do and outline your approach before making changes, then
     implement it in the working copy.
   - If the request is already implemented, verify that conclusion and do
     not make redundant code changes. Draft a response explaining the
     existing implementation and the evidence you found.
   - If the request is unreasonable, technically incorrect, contradictory,
     or unsafe, do not force a code change merely to satisfy it. Draft a
     response that clearly explains the concern and, when useful, proposes
     an alternative.

   In either no-code-change case, responding on the relevant issue discussion is
   the work product. Before running any `tod` command that changes OneDev state, show the exact planned action and obtain explicit user consent.

   **Push before publishing code-dependent updates.** If a OneDev state
   change relies on code being submitted — for example, a comment such as
   "Done the work" or any update claiming that a fix has been implemented —
   draft it and obtain consent now, but do not post it yet. Record the deferred
   action so it can be applied only after the code has been pushed and the pull
   request has been opened or updated. State changes that do not depend on
   submitted code may be made immediately after obtaining consent.

   When code was changed, leave the working copy on the issue branch with the
   implementation and any deferred OneDev actions ready for a separate
   submission request. No submission is needed when the outcome is only a
   discussion response.
