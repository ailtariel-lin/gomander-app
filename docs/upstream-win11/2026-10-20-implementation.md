# Upstream synchronization and Windows 11 support

## Goal and acceptance

Merge upstream `lazylabz/gomander-app` main at `463b511` (v1.10.0) into the personal fork starting at `dc1de59`, preserving its ConPTY configuration, clipboard integration, and manual release workflow. Then enable and verify ConPTY on Windows 11 without changing the Windows 10 allowlist behavior or Unix execution.

Windows verification must cover terminal output, reload, startup failure, stopping a command with descendants, and releasing its listening port. Preserve useful diagnostics when stopping fails. Do not run the user's application or replace the running desktop executable during verification.

## Phase 1: upstream merge

- Merge all 38 upstream commits on an integration branch.
- Resolve the About modal import conflict by retaining the fork's clipboard use case alongside upstream's release store and use cases.
- Review automatic merges involving command output, contracts, localization, and generated bindings.
- Install the repository's pinned Go and pnpm tools in a user-local tools directory if unavailable.
- Run frontend tests, typecheck, lint and build, plus Go tests and vet. Check that the fork-specific regression tests remain present and pass.
- Record verification and commit the merge before Windows 11 changes.

## Phase 2: Windows 11

- Add Windows 11 workstation classification and an explicit ConPTY allowlist entry in the personal build.
- Extend Windows terminal tests to execute on either enabled Windows workstation environment; retain Server and pre-ConPTY exclusions.
- Exercise the production runner against process-tree and local HTTP/reload cases. Adjust shutdown behavior only where a reproducible failure justifies it.
- Preserve taskkill output in stop errors.
- Run focused tests, then shared-runner regression checks and a Windows desktop build. Compile the Unix runner tests to check platform isolation.
- Review and commit separately, then integrate the verified commits into the fork.

## Status

- Phase 1: complete. All upstream commits merged; one textual conflict resolved. Frontend: 63 files / 312 tests pass, TypeScript and production build pass, Biome checks 250 files successfully. Fork runner and clipboard implementations remain intact. `go test ./... -count=1 -timeout=180s`, `go vet ./...`, and module verification pass. Generated Wails bindings retain upstream's whitespace convention.
- Phase 2: not started.

## Local verification environment

Go 1.24.5 (official archive verified against its SHA-256), pnpm 10.11.0, and Wails 2.10.2 are installed under the user's tools directories. Two dependencies rejected by `proxy.golang.org` were fetched directly from their source repositories and passed `go mod verify`. Local Git checkout conversion is set to `core.autocrlf=input` so Biome sees the LF endings already stored in Git; normalization introduced no source diff.
