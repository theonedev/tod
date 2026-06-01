---
name: work-on-issue
description: Set up the local checkout to work on a OneDev issue via the
  `tod` CLI — check for an existing open pull request linked to the issue
  (and delegate to work-on-pull-request when there is exactly one), otherwise
  create the issue branch on the server, switch to it locally with the right
  remote tracking, read the issue context, and implement the work. Use when
  the user asks to start, pick up, begin, or work on a OneDev issue.
---

# Work on a OneDev issue

This skill prepares the local checkout to work on a specified OneDev issue.
When the issue already has exactly one open pull request, it hands off to
[`work-on-pull-request`](../work-on-pull-request/SKILL.md). Otherwise it
ensures the issue branch exists, switches onto it (without clobbering
uncommitted work), reads the issue context, and implements the work.

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

1. **Check for an existing open pull request.** Query pull requests that
   include this issue and are still open:
   ```bash
   tod pr list --query 'includes issue "<issue-reference>" and open'
   ```
   Use the same reference form the user gave (e.g. `#123` or `PROJ-123`) inside
   the quotes.

   - **Exactly one match:** Stop this skill. Read and follow
     [`work-on-pull-request`](../work-on-pull-request/SKILL.md) using that
     PR's reference from the query result. The user asked to work on the
     issue, so use issue's description and comments as the primary 
     specification — run `tod issue get <issue reference>` and 
     `tod issue get-comments <issue reference>`, and download any embedded 
     resources from that issue text. Still gather other PR signals (comments, 
     code comments, builds) when they apply, but the issue text drives what 
     to implement.
   - **Zero or multiple matches:** Tell the user when there are multiple
     open PRs and ask which one to use, or continue below when there are
     none. Do not create an issue branch until this step is resolved.

2. **Create the issue branch on the server** and capture its name. The
   command is a no-op when the branch already exists; in either case it
   prints the branch name to stdout:
   ```bash
   tod issue create-branch <issue-reference>
   ```
   Save the output as `<issue-branch>`.

3. **Switch the local checkout to `<issue-branch>`.** First check the
   current branch:
   ```bash
   git symbolic-ref --short HEAD
   ```
   If it already equals `<issue-branch>`, skip ahead to step 4.
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

4. **Read the issue context.** Fetch the metadata and the discussion to
   learn what the work entails:
   ```bash
   tod issue get <issue-reference>
   tod issue get-comments <issue-reference>
   ```
   Without the issue context you cannot plan the work reliably, so do
   not skip ahead to step 5 from partial output.

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

5. **Plan and execute.** Summarize back to the user what the issue asks for
   and outline what you will do before making changes. Implement in the
   working copy. Follow [`using-tod`](../using-tod/SKILL.md) for write
   commands; use [`submit-issue-work`](../submit-issue-work/SKILL.md) when
   the user wants to push and open a pull request.
