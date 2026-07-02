<!-- Keep the title a Conventional Commit: feat: … / fix: … / docs: … -->

## What & why

<!-- What does this change and why? Link issues (Closes #N) or ROADMAP/BACKLOG/ISSUES items. -->

## How it was verified

- [ ] `task build` (or `go build ./...`)
- [ ] `task lint` (golangci-lint clean)
- [ ] `task test` (`go test -short ./...`)
- [ ] Integration verified (CI Postgres job, or `task test:docker` locally) — if the change touches migrate/query/verify
- [ ] Golden files regenerated if codegen output changed (`-update`)

## Notes

<!-- Behavior changes, follow-ups, or anything a reviewer should know. -->
