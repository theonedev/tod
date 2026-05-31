---
name: work-on-issue
description: Set up the local checkout to work on a OneDev issue via the
  `tod` CLI — create the issue branch on the server, switch to it locally
  with the right remote tracking, and read the issue's title, description,
  and comments to understand what to do. Use when the user asks to start,
  pick up, begin, or work on a OneDev issue.
---

# Work on a OneDev issue

This skill prepares the local checkout to start work on a specified
OneDev issue: it makes sure the issue branch exists on the server,
switches the working directory onto it (without clobbering uncommitted
work), and reads the issue context (title, description, comments) so the
agent knows what needs to be done.

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

Given an `<issue-reference>` (e.g. `#123`, `myproject#123`, or
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

3. **Read the issue context.** Fetch the metadata and the discussion to
   learn what the work entails:
   ```bash
   tod issue get <issue-reference>
   tod issue get-comments <issue-reference>
   ```
   Without the issue context you cannot plan the work reliably, so do
   not skip ahead to step 4 from partial output.

   Treat the title and description as the primary specification of the
   work, and the comments as supplementary context (clarifications,
   constraints, hints from collaborators, etc.).

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

4. **Summarize and plan.** Briefly summarize back to the user what the
   issue asks for and outline how you intend to address it before making
   any code changes.
