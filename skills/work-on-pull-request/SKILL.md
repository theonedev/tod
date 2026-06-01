---
name: work-on-pull-request
description: Set up the local checkout to address concerns on a OneDev pull
  request via the `tod` CLI — verify the working directory is the PR's source
  project, check out the PR branch, inspect feedback (PR comments, code
  comments, linked issue comments, failed builds, or the user's prompt), then
  fix code locally and draft or post OneDev replies. Replies that claim a code
  fix must not be posted until after `submit-pull-request-work` pushes. Use
  when the user asks to work on, address, fix, or follow up on a pull request
  (e.g. review feedback, failing CI, or requested changes).
---

# Work on a OneDev pull request

We work on a pull request when we need to address concerns raised by someone
or the OneDev system against the PR. Those concerns can come from PR
comments, line-anchored code comments, corresponding issue description and comments,
failed builds of the PR, or from the user's prompt alone.

This skill prepares the local checkout on the PR's source branch, gathers the
relevant feedback, and guides you to take the right actions — including code
changes and replies on OneDev. **Push before publish:** any reply or resolve
that implies the concern is fixed in the PR must wait until commits are on the
server (via [`submit-pull-request-work`](../submit-pull-request-work/SKILL.md)),
so reviewers are not notified of a fix they cannot see yet.

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

3. **Inspect the concern.** Use what the user pointed at; when they did not
   specify a source, gather every signal that applies:

   | Concern source | How to inspect |
   |----------------|----------------|
   | User prompt only | Treat the prompt as the specification; skip rows that do not apply. |
   | PR comments | `tod pr get-comments <pr-reference>` |
   | Line-anchored code comments | `tod pr get-code-comments <pr-reference>` — note `id`, file, line range, resolution state, and replies. |
   | Linked issue description and comments | `tod issue list --query 'fixed in pull request "<pr-reference>"'` → `tod issue get <issue-ref>` and `tod issue get-comments <issue-ref>` for each match |
   | Failed PR builds | `tod pr get-builds <pr-reference>` — for each failed build, follow [`investigate-build-failure`](../investigate-build-failure/SKILL.md) |

   For context on what changed, read the current patch when code edits are
   likely:
   ```bash
   tod pr get-patch <pr-reference>
   ```
   Fetch full files when the patch alone is insufficient:
   ```bash
   tod pr get-file-content <pr-reference> <path>
   ```

   **Download and inspect embedded resources.** PR descriptions, general
   comments, code comments, issue description and comments often link to
   screenshots, logs, or other files. Text alone is not enough when links
   are present:

   - Find image and file links (`![alt](url)` and `[label](url)`) in every
     text source you read above.
   - For each URL, save it with the URL **exactly** as written:
     ```bash
     tod download <resource-url> <output-file>
     ```
   - Open images with the Read tool; read other files as needed.

4. **Plan and execute.** Summarize the concern back to the user and outline
   what you will do before making changes. Then work in two phases:

   **Phase A — Local work**

   - Implement any code changes in the working copy.
   - For each OneDev reply or resolve you intend to send, decide whether it
     **depends on pushed code** (see table below). When it does, **draft** the
     text and show it to the user per [`using-tod`](../using-tod/SKILL.md)
     consent rules, but **do not post it yet** — it belongs in
     [`submit-pull-request-work`](../submit-pull-request-work/SKILL.md) step 6
     after a successful push.
   - When the user wants fixes on the server (or you drafted push-dependent
     replies), run `submit-pull-request-work` before treating the PR follow-up
     as complete. A single request such as "fix review feedback and submit"
     means: finish phase A, then run that skill (including posting deferred
     replies in its step 6).

   | Reply depends on pushed code? | Examples | Where to post |
   |------------------------------|----------|---------------|
   | **Yes** | "Fixed in …", "addressed your feedback", `resolve`, replies explaining a code change | Draft in phase A; post in `submit-pull-request-work` step 6 after push |
   | **No** | Clarification, "will fix", questions, disagreement without claiming a fix is already in the PR | Phase A — follow consent rules and post immediately |

   Match the channel when posting (phase A or submit step 6):
   - General PR feedback → `tod pr add-comment <pr-reference> "<reply>"`
   - Line-anchored thread → `tod code-comment add-reply <comment-id> "<reply>"`
   - Concern addressed in code → `tod code-comment resolve <comment-id> --note "<why>"` when appropriate
   - Issue-side discussion → `tod issue add-comment <issue-ref> "<reply>"` when the thread lives on the issue

   **Phase B — Publish on OneDev**

   - Post only **push-independent** replies from the table above.
   - Do **not** run `add-comment`, `add-reply`, or `resolve` here when the
     message depends on code that is not yet on the server — use submit step 6
     instead.

