---
name: work-on-issue
description: Implement work for a OneDev issue. Use when the user asks to start, pick up, or continue issue work.
---

# Work on a OneDev issue

Set up the issue branch, read the relevant context, and implement the work.

## Prerequisites

- `tod` is installed and configured.
- The current repository belongs to the issue's project.

## Session handoff

This workflow pairs with `submit-issue-work`. Hand off work within the **same
chat session** using two channels:

| Work product | Handoff |
|--------------|---------|
| Code changes | Uncommitted changes on the issue branch in the working copy |
| Issue comments | Drafted comment text held in session as `<saved-issue-comments>` |

When you draft issue comments, treat each one as **saved for later
submission** -- not merely presentation text. Keep the exact wording you intend
to post. A later `submit-issue-work` run in this session must be able to
retrieve `<saved-issue-comments>` and post them in its step 7.

Do not write comment drafts to OneDev in this workflow.

## Stop on error

Run the workflow sequentially. On any command failure, missing required
output, or failed precondition, stop immediately, report the command and
error, and wait for the user. Do not continue, repair state beyond the
current step, or retry silently.

## Workflow

Given an optional `<issue-reference>` (e.g. `123`, `#123`, `myproject#123`,
or `PROJ-123`):

1. **Resolve the issue reference.** If the user prompt or session context
   already provides `<issue-reference>`, use it. Otherwise derive it from the
   working directory:
   ```bash
   tod issue current-reference
   ```
   Save non-empty output as `<issue-reference>`. If the output is empty, stop
   and report that the issue reference could not be derived.

2. **Prepare the checkout.** Prepare the checkout to work on the issue,
   unless the user explicitly asks not to, or wants to switch to a different
   checkout.

   Check out the issue branch locally:
   ```bash
   tod issue checkout <issue-reference>
   ```
   The command creates the issue branch on the server when necessary,
   switches the local checkout to it, and sets up remote tracking. If it
   fails because the working directory has uncommitted changes, stop and ask
   the user to commit or stash them before re-running the skill.

3. **Determine the work specification.** The work may come from the user's prompt directly,
   from someone's feedback on the issue, or from the issue itself (title and description).

   Even when the prompt is the primary specification, do not proceed from the
   prompt alone. The issue title, description, and comments are important
   context, and can be retrieved as below:

   | Context source | How to inspect |
   |----------------|----------------|
   | Issue metadata, title, and description | `tod issue get <issue-reference>` -- note title, description, submitter, and other issue properties|
   | Issue comments | `tod issue get-comments <issue-reference>` |

   Then proceed to get your own login name:
   ```bash
   tod get-login-name
   ```

   Match your login name against the issue submitter, roles indicated in user prompt 
   if there are any, and author of each issue comment to understand your role, position, 
   and previous involvements in the context.

   When the work is to investigate a build failure or fix a failed build,
   treat the relevant `<build-reference>` as required context for assessment.

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
   current code and behavior before deciding whether code changes are needed.

   For failed build investigation/fix work only:

   - Gather and examine build evidence before planning code changes. Given
     the relevant `<build-reference>`:
     ```bash
     tod build get <build-reference>
     tod build get-log <build-reference>
     ```
   - Read the build detail and log content carefully to identify the failure.
   - If the log contains a statement like
     `Dependency build is required to be successful but failed: <dependency-build-reference>`,
     get the dependency build detail. If the dependency build is cancelled, do
     not investigate or fix it; draft an issue comment explaining that the
     relevant dependency build was cancelled. If its commit hash is the same as the
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

   The work product may be code changes, issue comments explaining the decision,
   or both. Implement any needed code changes in the working copy, and draft
   every issue comment that should be posted, including explanatory responses,
   status updates, or any update claiming that a fix has been implemented. Do
   not post comments in this workflow.

   Save the exact text of each drafted comment in `<saved-issue-comments>`
   (see **Session handoff**). If a comment mentions a pull request, use the
   form `pr #<pr number>`. When someone should be mentioned in comments or
   notes, mention each user only once across all drafted comments and notes
   for this submission -- not in every draft. Draft all issue comment text in
   Markdown, as `tod issue add-comment` posts Markdown text.

   Leave the working copy on the issue branch with all work ready for
   `submit-issue-work`.
