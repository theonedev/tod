---
name: edit-build-spec
description: Author or update `.onedev-buildspec.yml` for OneDev CI/CD. Use when the user asks to create, configure, or modify a OneDev build spec.
---

# Edit OneDev CI/CD spec

Author or update `.onedev-buildspec.yml` for the current OneDev project.

## Prerequisites

- `tod` is installed and configured.
- The current directory is the project root where
  `.onedev-buildspec.yml` belongs.

## Workflow

1. **Fetch the spec schema first.** Before editing anything, get the authoritative
   spec schema so the generated YAML matches the server's expectations:
   ```bash
   tod build get-spec-schema
   ```
2. **If a CI/CD spec already exists, validate and upgrade it first:**
   ```bash
   tod build check-spec
   ```
   This both validates and upgrades the file to the latest version when
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

When writing or modifying `.onedev-buildspec.yml`:

1. A checkout step is generally needed to clone repository into job working directory
2. Always call `tod build get-spec-schema` first so the YAML you produce matches
   the spec schema.
3. If a `command` step is used, enable "run in container" unless the user
   explicitly asks otherwise.
4. Different steps run in isolated environments and only share the job working directory.
   Installing dependencies in one step and running commands that rely on them
   in another will not work. Keep them in a single step unless the user
   explicitly wants them separated.
5. If a `cache` step is used:
   1. Place it **before** the step that builds or tests the project.
   2. If the project has lock files (`package.json`, `pom.xml`, etc.):
      1. Configure checksum files using these lock files.
      2. Configure the upload strategy as `UPLOAD_IF_NOT_HIT`.
6. To pass files between jobs, have the producing job publish them with the
   publish-artifact step, and let consuming jobs pull them in via a job
   dependency.
7. Call `tod build check-spec` **before** editing an existing CI/CD spec to
   make sure it is valid and up to date.
8. Inspect the relevant project files to decide which docker image and
   commands to use when the user asks you to set up build or test steps.
9. Call `tod build check-spec` **again after** editing so the final spec is
   confirmed valid.
