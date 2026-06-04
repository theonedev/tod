---
name: work-on-issue
description: Implement work for a OneDev issue. Use when the user asks to
  start, pick up, begin, or work on a OneDev issue.
---

# Work on a OneDev issue

This skill prepares the local checkout to work on a specified OneDev issue.
It ensures the issue branch exists, switches onto it (without clobbering
uncommitted work), reads the issue context (or uses the user's prompt when
it specifies the work), and implements the work.

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config set` if needed).
- The current working directory is inside a git repository pointing at
  the OneDev project that owns the issue.

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

   **Download and inspect embedded resources.** Descriptions and comments
   are often markdown with screenshots, mockups, logs, or other files.
   Text from `tod issue get` / `get-comments` alone is not enough when
   links are present — you must download and check each attachment:

   - Find image and file links in the description and every comment
     (`![alt](url)` and `[label](url)`).
   - For each URL, save it locally using the URL **exactly** as it
     appears in the markdown (do not rewrite or normalize it):
     ```bash
     tod download <resource-url> <output-file>
     ```
   - Open images with the Read tool; read other downloaded files as
     needed. Do not skip this step when attachments are linked — they
     often carry requirements or repro steps that are not spelled out in
     plain text.

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
   the work product. Follow [`using-tod`](../using-tod/SKILL.md) consent
   rules before posting it. For any other OneDev state change during the
   work (issue state, logged time, etc.), follow the same consent rules.

   **Push before publishing code-dependent updates.** If a OneDev state
   change relies on code being submitted — for example, a comment such as
   "Done the work" or any update claiming that a fix has been implemented —
   draft it and obtain consent now, but do not post it yet. Defer it to
   [`submit-issue-work`](../submit-issue-work/SKILL.md), where it is applied
   only after the code has been pushed and the pull request has been opened
   or updated. State changes that do not depend on submitted code may be
   made immediately after following the consent rules.

   When code was changed and the user wants it pushed and opened as a pull
   request, use
   [`submit-issue-work`](../submit-issue-work/SKILL.md). Do not run the
   submission workflow when the outcome is only a discussion response.
