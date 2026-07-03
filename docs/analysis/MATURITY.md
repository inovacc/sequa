# sequa — Project Maturity Assessment

- **Project:** `github.com/inovacc/sequa` — Go 1.26.4 CLI + embeddable library (Postgres migrations, SQL client/REPL, type-safe codegen, schema verify)
- **Stage:** **4 — Production**
- **Weighted score:** **90.7 / 100** (Σ 3176 ÷ 35 weights)
- **Confidence:** **High** — 10/10 dimensions measured, not estimated (coverage pulled from the CI artifact, `govulncheck`/`gocognit`/`-race` run, CI status verified)
- **Date:** 2026-07-03
- **Baseline:** first assessment — no prior `MATURITY.md`, so no delta.

**Read the score honestly.** 90.7 reflects genuine *engineering discipline* — a green required-CI gate, automated release, 0 reachable vulns, race-clean code, comprehensive docs, zero tech debt. It does **not** reflect a proven production *track record*: the project is **7 days old** (87 commits, 100% within 30 days) and **pre-1.0** (`v0.1.0`). "Production" here means production-grade *practices*; "battle-tested" it is not — which is exactly what the Stability grade captures.

## Scorecard

| Dimension | Weight | Grade | Evidence (measured) |
|---|:---:|:---:|---|
| Testing & Coverage | 5 | **B** | CI coverage **66.0%** (artifact), 51% short-mode; unit+integration+golden; deterministic. Below the 80% A-bar; dead zones `cmd/sequa` 0.0%, `internal/cli` 31.7%. |
| CI/CD & Release | 4 | **A** | 4-job CI (build · lint · govulncheck · real-Postgres integration), all **required** on protected `main`; 8/8 recent runs green; GoReleaser + `v0.1.0` published; Dependabot active. |
| Security | 4 | **A** | `govulncheck` 0 reachable vulns; fully parameterized SQL ($N binds); DSN redaction; input validation. Gap: no CI secret scan; broad gosec G304 exclusion. |
| Operational Readiness | 4 | **B** | `log/slog` JSON→stderr; 51 `%w` wraps; non-zero exits; crash-recovery (`reconcileHistory`, `context.WithoutCancel`). Gap: no graceful shutdown (0 `signal.NotifyContext`). |
| Correctness & Robustness | 4 | **A** | `go test -race -short` clean; `go vet` clean; 0 panics; every rows-query has `defer Close`+`Err()`. Gaps: `-race` not in CI; ISS-1 residual crash window. |
| Architecture & Boundaries | 3 | **B** | No import cycles; clean top-down DAG; `Engine`/`DBTX` seams; clean `pkg/sequa` facade. Gaps: no ADRs; `query.go` 703 LOC (2.4× next). |
| Code Quality & Tech Debt | 3 | **A** | `gocognit` 0 over 15 (ceiling 14); **0** TODO/FIXME, **0** `nolint`; gofmt/vet clean. Only wart: `query.go` size. |
| Dependency & Supply-chain | 3 | **B** | 6 direct deps pinned + `go.sum`; vuln-clean; Dependabot working. Gap: 432 transitive modules (200 outdated, 12 deprecated) via `xo/usql` cloud drivers; no license scan. |
| Stability & Change Mgmt | 3 | **B** | 0 reverts; semver + Keep-a-Changelog; consolidated backlog; PR-gated `main`. Caps: 7-day-old (no churn track record); pre-1.0; 0 required reviews. |
| Documentation | 2 | **A** | README + DESIGN + guide + ROADMAP/BACKLOG/ISSUES + specs + index; 95% exported doc-comment; all 12 internal links resolve. Gaps: no ADRs; no diagram; 3 undoc'd `Engine` methods. |

## Weak points ranked by **leverage** (fan-out × weight ÷ effort — *not* severity)

| # | Fix | Fan-out | Wt | Effort | Leverage |
|:--:|---|:--:|:--:|:--:|:--:|
| 1 | gitleaks secret-scan in CI | 3 | 4 | S | **12** |
| 2 | Coverage gate (ratchet from 66%) | 2 | 5 | S | **10** |
| 3 | `-race` in CI (unit + integration) | 2 | 4 | S | **8** |
| 3 | Pin `govulncheck` version | 2 | 4 | S | **8** |
| 3 | `signal.NotifyContext` (graceful shutdown) | 2 | 4 | S | **8** |
| 6 | **Postgres/testcontainers harness** | 3 | 5 | M | **7.5** ← the one thing |
| 7 | License scan in CI | 2 | 3 | S | 6 |
| 8 | Split `query.go` (703 LOC) | 3 | 3 | M | 4.5 |
| 8 | ADRs → `docs/adr/` | 3 | 3 | M | 4.5 |
| 14 | Raise coverage 66%→80% | 1 | 5 | M | **2.5** (severe but isolated) |
| 15 | ISS-1 txn crash-window fix | 1 | 4 | M | **2** (severe but isolated) |

Deliberate inversion: **ISS-1** (a data-integrity crash window) and **coverage→80%** (heaviest dimension) rank *below* the cheap CI wins because their fan-out is 1 — a moderate fix that unblocks three dimensions beats a severe fix that unblocks nothing else.

## Route — Stabilize → Harden → Mature

### Phase 1 — Stabilize (this week; independent S-fixes, parallelizable)
1. **`-race` in CI** (unit + integration jobs) → Correctness A-→A, Testing. First action: add `-race` to both `go test` calls in `ci.yml`.
2. **gitleaks + pin `govulncheck`** → Security, CI/CD, Ops. First action: add a pinned gitleaks job + `.gitleaks.toml`; pin govulncheck to a release tag.
3. **Coverage gate (ratcheting)** → Testing, CI/CD. First action: fail CI below a 66% floor; ratchet +2%/PR toward 80.
4. **`signal.NotifyContext`** at `cmd/sequa/main.go`/`root.go` → Ops, Correctness. First action: replace `context.Background()` in `Execute` with a signal-cancelled context.
5. **Align GitHub Action versions** (`release.yml` vs `ci.yml`) → CI/CD. First action: bump/pin action refs uniformly (ideally SHAs).
6. **ISS-1 transaction fix** (sequence after the quick wins) → Correctness. First action: wrap the history insert in the migration's `*sql.Tx`; add a fault-injection test once the harness lands.

### Phase 2 — Harden
- **2.1 Postgres/testcontainers harness** (`internal/dbtest`, skip when Docker absent) — the pivot; enables local coverage/`-race`/DB-error verification. *Flag: run in a dev-container/WSL where Application Control doesn't block binaries.*
- **2.2 Raise coverage 66%→80%** (depends on 2.1) — table-driven tests for `cmd/sequa` (0.0%), `internal/cli` (31.7%); ratchet the gate up.
- **2.3 Trim/gate `xo/usql` drivers** behind a `query` build tag → shrink the 432-module surface (Deps, Security, Code-Quality). Effort L — do not front-load.
- **2.4 License scan + release signing/SBOM/SLSA** (go-licenses, cosign, syft).
- **2.5 Narrow gosec G304** to per-call justifications.

### Phase 3 — Mature
- **3.1 ADRs** from DESIGN.md §3 → `docs/adr/` (Arch, Docs, Stability).
- **3.2 Split `internal/codegen/query.go`** (703 LOC) into cohesive files.
- **3.3 Architecture diagram** (mermaid) + document the 3 `Engine` interface methods.
- **3.4 Pre-1.0 → 1.0 API stability** (`docs/COMPATIBILITY.md`, a deprecation window, a review gate as contributors join). Note: the churn track-record sub-gap resolves only with calendar time.

## The one thing

**A Postgres / testcontainers integration harness.** It is the single fix whose fan-out dominates the graph — the shared root behind three of the four heaviest dimensions: **Testing (w5)** gets a second reproducible coverage path and the tractable route into the CLI/DB dead zones (66%→80%); **Correctness (w4)** gets a real-Postgres path to run `-race` on integration code; **Ops (w4)** gets local DB-error verification instead of CI-only inference. The cheap CI-hardening wins ship first (they gate correctness/security and cost ~nothing), but the harness is the pivot that turns the remaining B's into A's.
