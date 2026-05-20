# ecspresso v3 plan

Tracking list of breaking changes and cleanups planned for ecspresso v3. Work lands on the `pre-v3` branch until v3 is released.

## Breaking changes

### Bump Go module path to `/v3`

The Go module is currently `github.com/kayac/ecspresso/v2`. v3 bumps it to `/v3`.

- Update the `module` directive in `go.mod`.
- Rewrite every internal import (`github.com/kayac/ecspresso/v2/...` → `/v3/...`).
- Users who consume ecspresso as a library (e.g. `import "github.com/kayac/ecspresso/v2"`) must update their import path.

This is the canonical signal that the v3 line has started.

### YAML configuration must be valid YAML before template rendering

Already in `pre-v3` (#1023).

The two-pass config loader runs a YAML parser on the raw file in pass 1 to extract `plugins`, so YAML configs must be valid YAML on their own. Template literals that begin a scalar must be quoted.

```yaml
# NG in v3 (worked in v2)
region: {{ must_env `AWS_REGION` }}

# OK
region: "{{ must_env `AWS_REGION` }}"
```

JSON and Jsonnet configs are unaffected.

### Remove deprecated `filter_command` config field

Originally deprecated in v2 (#469, see [docs/v1-v2.md](v1-v2.md)). Removal in v3.

- Replacement: `ECSPRESSO_FILTER_COMMAND` environment variable, or the `--filter-command` CLI flag.
- Items to delete:
  - `config.go`: `Config.FilterCommand` field, the `LogWarn` branch in `Restrict`, and the `OverrideByCLIOptions` write that copies CLI → config.
  - `config_test.go`: `TestFilterCommandDeprecated` and its `FilterCommandTests` table.
  - `tests/filter_command.yml` fixture.

The CLI flag and env-var pathways (`CLIOptions.FilterCommand`, `App.FilterCommand()`) are kept.

### Remove deprecated `exec` flags

Replaced by the `exec portforward` subcommand. The legacy flags have been hidden since v2.

- Items to delete:
  - `exec.go`: `ExecRunOption.{PortForward,LocalPort,Port,Host,L}` fields and the `case run.PortForward:` branch in `App.Exec` (along with its `LogWarn`).
  - `cli_test.go`: the "exec: deprecated --port-forward (backward compat)" test case (around line 980).

### Remove deprecated `tasks` flags

Replaced by the `tasks find`, `tasks stop`, and `tasks trace` subcommands. The legacy flags have been hidden since v2.

- Items to delete:
  - `tasks.go`: `TasksListOption.{DeprecatedFind,DeprecatedStop,DeprecatedForce,DeprecatedTrace}` fields and the three `LogWarn` branches in `App.Tasks`.
  - `cli_test.go`: the "tasks: deprecated flags (backward compat)" test case (around line 913).

### Remove dead `create` dispatch case

The `create` subcommand was removed from `CLIOptions` back in v2, so the Kong parser rejects it before dispatch. The `case "create":` branch in `dispatchApp` (`cli.go:161-162`) is unreachable dead code and can go.

### Drop the `--no-unified` diff path and the `kylelemons/godebug` dependency

`DiffOption.Unified` is `default:"true"` and every callsite — internal (`deploy.go:137`) and every test in `diff_test.go` / `cli_test.go` — uses `Unified: true`. The non-unified branch survives only as a `default:` arm in `diffServices` / `diffTaskDefs` (`diff.go:214,257`) and the express-mode equivalent. It is the sole reason for the `github.com/kylelemons/godebug/diff` dependency, which has had no upstream pushes since 2022-06.

#### Plan

- Delete the `Unified` field from `DiffOption` (and the `--[no-]unified` CLI flag).
- Delete the `default:` (non-unified) branch in `diffServices`, `diffTaskDefs`, and the express-mode diff helpers.
- Drop the `github.com/kylelemons/godebug/diff` import here and remove `github.com/kylelemons/godebug` from `go.mod` / `go.sum`.
- Strip `Unified: true` from internal callsites and test fixtures.

#### Compatibility

- Users invoking `ecspresso diff --no-unified` will get a flag-parse error. The same goes for `--unified` (no-op today, but parses fine — that also goes away).
- Library callers passing `DiffOption{Unified: true, ...}` need to drop the field.
- `hexops/gotextdiff` (unified output) stays. It is also stale (last push 2023-09) but is a self-contained copy of the Go language server's internal diff packages, so there is no real maintenance pressure; replacement can wait for a future major.

### Move the GitHub Action to a dedicated repository

The GitHub Action (`action.yml`) currently lives in this repository, alongside the CLI source. From v3 the Action will live in its own repository so it can be versioned and released independently of the CLI.

#### Motivation

- **Avoid noisy Dependabot updates for users.** Today the action ships with the CLI, so every CLI release bumps `kayac/ecspresso`'s tags and Dependabot proposes `uses: kayac/ecspresso@vX.Y.Z` updates to every workflow — even when `action.yml` itself has not changed. Splitting the repo means the action only releases when it actually changes, and CLI patch releases stop generating churn in users' Dependabot PR queues.
- **Release the action by tag, not by branch.** Users currently pin against the `v2` branch (`uses: kayac/ecspresso@v2`), which is fine but does not match how third-party actions are normally versioned. In the new repo we want to release with proper semver tags (`@v3`, `@v3.1.0`, etc.) so Dependabot pins behave predictably and users can opt in to a specific action revision.
- **Independent CI for the action.** The action's tests no longer need to ride on top of the CLI repo's CI nor be force-pushed to a special `v2-action-testing` branch — the new repo runs its own workflow directly on PRs.

#### Plan

- Repository name: **TBD**.
- Migration for users: `uses: kayac/ecspresso@v2` → `uses: <new-org-or-owner>/<new-repo>@v3`.
- Items to move out:
  - `action.yml` at the repo root.
  - The `v2-action-testing` branch workflow (currently the CI gate for `action.yml` changes — see [CLAUDE.md](../CLAUDE.md)). The new repo will run its own CI directly on PRs, so the `v2-action-testing` force-push convention goes away.
- Items to update in this repo on cutover:
  - README "GitHub Actions" section (currently around L142) — point users to the new repo.
  - CLAUDE.md — drop the `v2-action-testing` push instruction.

#### Keep `action.yml` as a forwarder, do not delete it

If we delete `action.yml` from this repo and just cut the v3 tag, Dependabot will still propose `uses: kayac/ecspresso@v2` → `@v3` to existing users (it looks at the repo's tags, not whether `action.yml` exists at that tag). Merging that PR would break their workflow with `unable to resolve action 'kayac/ecspresso@v3'`.

To avoid that, keep `action.yml` in this repo at v3 as a thin composite action that just calls the new repo, plus a deprecation notice:

```yaml
# action.yml (sketch)
name: ecspresso (moved)
description: |
  The ecspresso GitHub Action has moved to <new-org-or-owner>/<new-repo>.
  This wrapper forwards calls; please update your workflow.
inputs:
  version:
    required: false
  version-file:
    required: false
runs:
  using: composite
  steps:
    - run: echo "::warning::kayac/ecspresso action is deprecated; use <new-org-or-owner>/<new-repo>@v3 instead"
      shell: bash
    - uses: <new-org-or-owner>/<new-repo>@v3
      with:
        version: ${{ inputs.version }}
        version-file: ${{ inputs.version-file }}
```

With this in place:

- Dependabot's `@v2` → `@v3` PR keeps working after merge.
- Users see a deprecation warning prompting them to switch the `uses:` line.
- The forwarder can be removed in a future major (or simply left in place indefinitely; it is cheap).

The new repo is the source of truth — keep the forwarder's `inputs` in sync with whatever inputs the new action accepts.

### Unify Duration handling and reinterpret plain numbers as seconds

`Duration` (`duration.go`) is currently defined inside the `ecspresso` package, so subpackages (notably `external/`) cannot import it. As a result `external.Config.Timeout` is a bare `int64` (seconds), which is inconsistent with the main `Config.Timeout` (a `*Duration` that accepts `"10m0s"` etc.).

#### Plan

- Move `Duration` into a new package `github.com/kayac/ecspresso/v3/duration`. Both the main package and `external/` import it.
- Change `external.Config.Timeout` from `int64` to `duration.Duration` so plugin config can write `timeout: "30s"`, `timeout: "1h"`, `timeout: 30`, etc.
- Adjust the value semantics of `Duration` (`UnmarshalJSON` / `UnmarshalYAML`) so plain numbers — and pure-digit strings — are interpreted as seconds, not nanoseconds:

```go
case float64:
    d.Duration = time.Duration(value) * time.Second
case string:
    if n, err := strconv.ParseInt(value, 10, 64); err == nil {
        d.Duration = time.Duration(n) * time.Second  // "30" → 30s
        return nil
    }
    d.Duration, err = time.ParseDuration(value)       // "30s", "5m", "1h30m"
```

#### Nanosecond-input guard

Some v2 users may have written nanosecond values (e.g. `30000000000` to mean "30 seconds") under the old semantics. After the change, that input becomes 30 000 000 000 seconds (~951 years), which is clearly wrong.

To catch these without breaking realistic timeouts, emit a `slog.Warn` when the seconds-interpreted result exceeds 30 days (`30 * 24 * time.Hour`). 30 days is comfortably above any realistic ecspresso timeout and comfortably below the seconds-interpretation of typical nanosecond inputs, so the warning fires on the mistake but not on real configs. The warning shows both the raw input and the interpreted duration:

```text
duration value is unusually large for a timeout; interpreted as seconds (v2 interpreted plain numbers as nanoseconds)
  input=30000000000 interpreted=951323y23w hint="use \"30s\" / \"5m\" / \"1h\" string notation to make the unit explicit"
```

#### Compatibility

- Users who wrote `timeout: "10m0s"` (string) are unaffected.
- Users who wrote `timeout: 600` meaning "ten minutes" but got 600 ns in v2 are silently fixed.
- Users who deliberately wrote nanoseconds as a plain number now get a behaviour change, but they also get a clear warning at load time.

## Cleanups

### Unexport package-level tuning variables

`ecspresso.go:38-41` declares `delayForServiceChanged`, `refreshInterval`, `waiterMaxDelay`, and `spcIndent` as package-level `var`s. They look like test seams, but as exported (or even just package-level mutable) names they are part of the library's surface — callers can mutate them at runtime, which is rarely intentional.

- Audit each one for actual usage outside the package and outside tests.
- Unexport (lowercase) the ones used only internally / for tests.
- For anything that legitimately wants to be tunable, expose it through an `AppOption` builder instead of a raw `var`.

### `cliv2.go` / `ParseCLIv2` naming

File and function names date back to the v1 → v2 CLI parser switch (kingpin → kong). With v1 long gone, the `v2` suffix is just noise.

- Candidate: rename `cliv2.go` → `cli_parse.go` (or merge into `cli.go`), `ParseCLIv2` → `ParseCLI`.
- The `args[0] == "help"` shim that rewrites `help` to `--help` is technically v1 compatibility, but it is convenient and cheap; keep unless we want to be strict.

### docs/v1-v2.md

Migration notes from the v1 → v2 cut. Keep as historical reference, but link to it from this document so the v3 migration story is self-contained.

## Out of scope (intentionally kept)

- `App.FilterCommand()` method and the `--filter-command` CLI flag / `ECSPRESSO_FILTER_COMMAND` env var — these are the v2-blessed replacements for the config field above, not the deprecation targets.
- `exec portforward` / `tasks find|stop|trace` subcommands — these are the replacements, not legacy.
