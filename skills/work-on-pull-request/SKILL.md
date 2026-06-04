---
name: work-on-pull-request
description: Implement changes for a OneDev pull request. Use when the user
  asks to work on, address, fix, or follow up on a pull request.
---

# Work on a OneDev pull request

This skill prepares the local checkout to work on a specified OneDev pull
request. It verifies the source project, switches onto the PR's source branch
(without clobbering uncommitted work), reads the PR context (or uses the user's
prompt when it specifies the work), and implements the work.

The work may also require replies or resolutions on OneDev. **Push before
publish:** any reply or resolve that implies a code change is complete must
wait until the commits are on the server (via
[`submit-pull-request-work`](../submit-pull-request-work/SKILL.md)), so
reviewers are not notified of a change they cannot see yet.

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config set` if needed).
- The current working directory is inside a git repository whose OneDev
  remote points at the **source project** of the pull request (see step 1).

## Stop on error

**The steps below are sequential and each one depends on the previous
one succeeding.** If any command in the workflow fails — non-zero exit
code, an error message on stderr, empty output where output is
required, or a precondition check (e.g. wrong project, uncommitted
changes) that does not hold — you **must**:

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

Given a `<pr-reference>` (e.g. `42`, `#42`, `myproject#42`, or `PROJ-42`):

1. **Confirm the source project.** Read the PR metadata and the project
   inferred from the working directory:
   ```bash
   tod pr get <pr-reference>
   tod project current
   ```
   From `tod pr get`, note `<source-project>`. The output of `tod project current` 
   must equal `<source-project>`. If it does not, stop and tell the user.

2. **Check out the pull request.** `tod pr checkout` requires a clean
   working directory:
   ```bash
   git status --porcelain
   ```
   Any output means there are uncommitted changes; stop and ask the user to
   commit or stash them before re-running the skill.

   Then check out the PR head onto the source branch (when the PR is open
   and the cwd is the source project):
   ```bash
   tod pr checkout <pr-reference>
   ```
   Verify you are on `<source-branch>`:
   ```bash
   git symbolic-ref --short HEAD
   ```

3. **Determine the work specification.** The work may
   come from the user's prompt directly or from feedback on OneDev:

   | Work instruction source | Primary specification | OneDev context |
   |-------------------------|-----------------------|----------------|
   | User prompt specifies the work | The prompt (concrete task, scope, approach, or constraints beyond naming the PR) | Always use PR metadata and description; inspect the feedback sources identified by the prompt and any directly related discussion as supplementary context |
   | Prompt only names the PR | PR title and description | Comments, code comments |
   | Prompt generally asks to address feedback | Applicable PR comments and code comments | PR metadata, description |

   Even when the prompt is the primary specification, do not proceed from the
   prompt alone. The PR metadata and description fetched in step 1 are required
   context, and any feedback source the prompt points to must be fetched in
   full, including its replies. First determine the current login name:
   ```bash
   tod get-login-name
   ```
   Save the login name and match it against the author of each PR comment,
   code comment and reply. Treat matching comments as your own prior context 
   rather than independent collaborator feedback.

   | Context source | How to inspect |
   |----------------|----------------|
   | PR comments | `tod pr get-comments <pr-reference>` |
   | Line-anchored code comments | `tod pr get-code-comments <pr-reference>` — note `id`, file, line range, resolution state, and replies. |

   Read the current patch to understand what the PR already changes relative
   to its target:
   ```bash
   tod pr get-patch <pr-reference>
   ```
   When the patch alone is insufficient, inspect the corresponding files in
   the checked-out working copy.

   **Download and inspect embedded resources.** PR descriptions, general
   comments, and code comments often link to screenshots, logs, or other files. 
   Text alone is not enough when links are present:

   - Find image and file links (`![alt](url)` and `[label](url)`) in every
     text source you read above.
   - For each URL, save it with the URL **exactly** as written:
     ```bash
     tod download <resource-url> <output-file>
     ```
   - Open images with the Read tool; read other files as needed.

4. **Assess, plan, and execute.** Check the requested work against the current
   code and behavior before deciding that a code change is needed.

   - If the request is reasonable and not already implemented, summarize what
     you will do, outline your approach, and then implement the change.
   - If the request is already implemented, verify that conclusion and do not
     make a redundant code change. Draft a response explaining the existing
     implementation and the evidence you found.
   - If the request is unreasonable, technically incorrect, contradictory, or
     unsafe, do not force a code change merely to satisfy it. Draft a response
     that clearly explains the issue and, when useful, proposes an
     alternative.

   In either no-code-change case, responding in the relevant PR or code-comment 
   is the work product. Follow [`using-tod`](../using-tod/SKILL.md) consent 
   rules before posting it.

   **Push before publishing code-dependent updates.** If a reply or resolve
   relies on code being submitted — for example, "Fixed", "addressed your
   feedback", or an explanation of a code change — draft it and obtain consent
   now, but do not post it yet. Defer it to
   [`submit-pull-request-work`](../submit-pull-request-work/SKILL.md), where it
   is applied only after the code has been pushed. Replies that do not depend
   on submitted code, such as clarifications, questions, or disagreement, may
   be posted immediately after following the consent rules.

   Match the discussion channel when posting immediately or deferring an
   update:
   - General PR feedback → `tod pr add-comment <pr-reference> "<reply>"`
   - Line-anchored thread → `tod code-comment add-reply <comment-id> "<reply>"`
   - Concern addressed in code → `tod code-comment resolve <comment-id> --note "<why>"` when appropriate
   
   When code was changed and the user wants it pushed to the existing pull
   request, use
   [`submit-pull-request-work`](../submit-pull-request-work/SKILL.md). Do not
   run the submission workflow when the outcome is only a discussion response.
