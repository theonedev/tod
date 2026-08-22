---
name: fix-failed-build
description: Fix a prompt-specified failed OneDev build when the request has no issue or pull request context.
---

# Fix a failed OneDev build

Inspect the failed build from the user's prompt and implement the fix in the
current checkout.

## Prerequisites

- `tod` is installed and configured.
- The current repository belongs to the same OneDev project as the failed
  build.
- The current checkout is on a branch.

## Stop on error

Run the workflow sequentially. On any unrecoverable command failure, missing
required output, or failed precondition, report the command and error and
stop. Do not continue the workflow.

## Workflow

Given a required `<build-reference>` from the user prompt (e.g. `123`, `#123`,
`myproject#123`, or `PROJ-123`):

1. **Resolve the build reference.** Use the `<build-reference>` from the user
   prompt. If the prompt does not provide a build reference, stop and ask for
   the failed build reference.

2. **Verify the checkout context.** Do not prepare or switch checkouts.

   Confirm that the current checkout is on a branch:
   ```bash
   git symbolic-ref --short HEAD
   ```
   If the command fails or reports a detached HEAD, stop and report that the
   workflow requires a branch checkout.

   Confirm the current project:
   ```bash
   tod project current
   ```
   Save the output as `<current-project>`.

3. **Gather and examine build evidence.** Run:
   ```bash
   tod build get <build-reference>
   tod build get-log <build-reference>
   ```

   Read the build detail and log content carefully to identify the failure.
   Verify that the build belongs to `<current-project>` using the project
   field in the build detail when present. If the build detail indicates a
   different project, stop and report the mismatch.

4. **Assess and fix the failure.**

   - If the log contains a statement like
     `Dependency build is required to be successful but failed: <dependency-build-reference>`,
     get the dependency build detail. If the dependency build is cancelled, do
     not investigate or fix it; report that the relevant dependency build was
     cancelled. If its commit hash is the same as the current build,
     investigate or fix the dependency build failure instead; repeat this
     process for same-commit dependency build failures. If the dependency
     build's commit hash differs from the current build, conclude that the
     current build failure is caused by this dependency build.
   - If the log contains a statement like
     `[<report-name>]: found problems with severity <severity-level> or higher`,
     fetch the referenced problems report:
     ```bash
     tod build get-code-problems <build-reference> <report-name> <severity-level>
     ```
     Problems may point to workspace files, 1-based line ranges, or
     non-workspace artifacts used by the project.
   - If the log contains `[<report-name>]: <count> not passed test cases`,
     inspect the report and any relevant artifacts it references:
     ```bash
     tod build get-unit-test-report <build-reference> <report-name>
     tod build get-unit-test-report <build-reference> <report-name> --artifact <artifact-path> > <output-file>
     ```
   - Inspect referenced workspace files as necessary. Inspect
     `.onedev-buildspec.yml` when job configuration may be involved, and
     run below command to get its schema if you need to modify it:
     ```bash
     tod build get-spec-schema
     ```
   - If useful, inspect changes since the previous successful build:
     ```bash
     tod build get-changes-since-success <build-reference>
     ```

   Implement any needed code changes in the working copy. Do not stage,
   commit, push, or post comments in this workflow. If no code change is
   appropriate, report the reason.

5. **Leave the result in the working copy.** Run:
   ```bash
   git status --short
   ```
   Leave any fixes as uncommitted changes in the working copy on the current
   branch.
