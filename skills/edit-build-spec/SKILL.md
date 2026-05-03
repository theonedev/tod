---
name: edit-build-spec
description: Create or edit a OneDev build spec (`.onedev-buildspec.yml`) using the `tod` CLI to fetch the schema and validate the result. Use when the user asks to add a CI job, configure a OneDev build, set up caching, publish artifacts, or otherwise author `.onedev-buildspec.yml`.
---

# Edit OneDev build spec

This skill helps the agent author or modify `.onedev-buildspec.yml` for a
OneDev project.

## Prerequisites

- `tod` is installed and on `PATH` with a configured tod config file (run
  `tod config init` if needed).
- The current working directory is the project root (the directory that
  contains, or should contain, `.onedev-buildspec.yml`).

## Workflow

1. **Fetch the spec first.** Before editing anything, get the authoritative
   spec so the generated YAML matches the server's expectations:
   ```bash
   tod build spec
   ```
2. **If a build spec already exists, validate and upgrade it first:**
   ```bash
   tod build check-spec
   ```
   This both validates and rewrites the file to the latest version when
   needed.
3. **Inspect the project.** Read the files you need (package manifests,
   language version files, Dockerfiles, etc.) to understand what image and
   commands the job should use.
4. **Edit `.onedev-buildspec.yml`** applying the user's instruction and the
   rules below.
5. **Validate again** after editing:
   ```bash
   tod build check-spec
   ```
   Keep iterating until `tod build check-spec` succeeds.

## Authoring rules

When writing or modifying `.onedev-buildspec.yml`, remember:

1. Always call `tod build spec` first so the YAML you produce matches
   the server spec.
2. If a `command` step is used, enable "run in container" unless the user
   explicitly asks otherwise.
3. Different steps run in isolated environments but share the job workspace.
   Installing dependencies in one step and running commands that rely on them
   in another will not work. Keep them in a single step unless the user
   explicitly wants them separated.
4. If a `cache` step is used:
   1. Place it **before** the step that builds or tests the project.
   2. If the project has lock files (`package.json`, `pom.xml`, etc.):
      1. Add a "generate checksum" step before the cache step that computes a
         checksum of all relevant lock files and stores it in `checksum.txt`.
      2. Configure the cache key as `<keyname>-@file:checksum.txt@`.
      3. Configure load keys as `<keyname>`.
      4. Configure the upload strategy as `UPLOAD_IF_NOT_HIT`.
5. To pass files between jobs, have the producing job publish them with the
   publish-artifact step, and let consuming jobs pull them in via a job
   dependency.
6. Call `tod build check-spec` **before** editing an existing build spec to
   make sure it is valid and up to date.
7. Inspect the relevant project files to decide which docker image and
   commands to use when the user asks you to set up build or test steps.
8. Call `tod build check-spec` **again after** editing so the final spec is
   confirmed valid.
