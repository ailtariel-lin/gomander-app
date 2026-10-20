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
- Phase 2: Windows 11 classification and the personal-build allowlist are implemented. Native Windows 11 tests confirm ConPTY output, failure exit status, process-tree shutdown, port release, and repeated stop. Windows Server/domain-controller exclusions and the existing Windows 10 allowlist remain covered.

## Windows 11 findings and verification

- Windows 11 build 26200 emits a distinct initial console header followed by 999 empty viewport rows and a cursor-home sequence. The output adapter now filters this captured host initialization while retaining command output and subsequent blank lines. Windows 10 initialization filtering is unchanged.
- Failed `taskkill` calls now include their native output and arguments, preserving the original exit error for `errors.As`. The original user-reported `128` was not reproduced; no unverified shutdown algorithm replacement was made.
- An isolated harness used the production `DefaultRunner` and the existing project's `uv`/Uvicorn environment, serving only a temporary ASGI application on temporary ports. Three reloads passed both with and without HTTP keep-alive. An intentional startup failure also passed stop, `ProcessFinished`, empty running-command state, and port-rebind checks. Temporary diagnostic source and applications were removed.
- Windows 11 targeted tests and the smoke scenarios pass. Final `go test ./... -count=1 -timeout=180s` passes. Linux amd64 and Windows arm64 runner tests cross-compile successfully. The Wails 2.10.2 production build generated `build/bin/gomander-v1.10.0-win11-amd64.exe`; generated bindings and module files stayed unchanged.
- The packaged desktop executable starts and exposes `/discovery` successfully with isolated `APPDATA`, `LOCALAPPDATA`, and WebView2 storage. It creates its own diagnostic database. The hidden smoke instance was terminated after verification; the user's existing instance and database were not replaced.
- Final `go vet ./...`, `gofmt`, and diff checks pass. Both phases are ready for fork synchronization.
- Windows 10 runtime execution is unavailable on this Windows 11 host; preserved code paths and classification tests are checked locally. Linux/macOS runtime tests and race-detector execution (no C compiler installed) are not claimed.

## Local verification environment

Go 1.24.5 (official archive verified against its SHA-256), pnpm 10.11.0, and Wails 2.10.2 are installed under the user's tools directories. Two dependencies rejected by `proxy.golang.org` were fetched directly from their source repositories and passed `go mod verify`. Local Git checkout conversion is set to `core.autocrlf=input` so Biome sees the LF endings already stored in Git; normalization introduced no source diff.
