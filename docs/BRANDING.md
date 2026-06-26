# the-migrator — Branding Names

> One tool to **migrate, query, and generate** type-safe Go from a single SQL schema — where your migrations are the source of truth.

- **Domain:** Go database tooling — schema migrations, an ad-hoc SQL client/REPL, and migration-driven type-safe code generation.
- **Audience:** Go backend developers managing SQL databases (PostgreSQL first; MySQL/SQLite next) in their daily workflow.
- **Thesis:** the schema is defined exactly **once** (as migrations); `migrate`, `query`, and `generate` all derive from that single source — so the generated Go code can never drift from the applied schema.
- **Lineage:** migration engine = `golang-migrate` + a goose-style UX; query = `usql` (REPL + CLI); codegen = a first-party engine adapted from `sqlc`'s Postgres parse→catalog→codegen path.

## Project Name Candidates

| # | Name | Rationale | Conflict check |
|---|------|-----------|----------------|
| 1 | **the-migrator** *(current)* | Descriptive, unambiguous — but "migrator" undersells the query + codegen unification. | Clear |
| 2 | **Cairn** ⭐ | A trail-side stack of stones — each migration a stone; the stack *is* your schema's path and waymarker. Short, brandable, perfectly on-metaphor for ordered migrations as the canonical record. | Low — no major dev tool |
| 3 | **Strata** | Geological layers; migrations are ordered strata that compose the live schema. | Low–moderate |
| 4 | **SchemaForge** | The schema is forged once; migrations and Go code are struck from the same mold. | Low |
| 5 | **Granite** | A single bedrock source of truth — solid, immovable schema under every verb. | Low |
| 6 | **Plinth** | The pedestal everything stands on: the schema base beneath migrate/query/generate. | Low |
| 7 | **Tributary** | Three streams — migrate, query, generate — feeding one river of truth. | Low |
| 8 | **Keystone** | The stone that locks the arch; the schema all verbs depend on. | ⚠️ KeystoneJS / OpenStack Keystone |
| 9 | **Lodestar** | The guiding star migrations steer codegen by. | ⚠️ ChainSafe Lodestar (eth client) |
| 10 | **Codex** | The single book of record — and a wink at *code* generation. | ⚠️ generic / OpenAI Codex |
| 11 | **Sequa** | Portmanteau of SQL + sequel/aqua; short, abstract, brandable. | Low |
| 12 | **Mason** | Lays your schema brick by brick on a firm foundation. | Moderate (common word) |

**Recommendation:** **Cairn** — distinctive, low-conflict, and the stacked-stones metaphor maps cleanly onto stacked migrations acting as the schema's single waymarker. Runner-up: **Strata**.

## Feature Names

| Feature | Current Name | Branded Options |
|---|---|---|
| Schema migrations | `migrate` | Ascent, Waymark, Uplift |
| SQL client + REPL | `query` | Scout, Probe, Console |
| Type-safe codegen | `generate` | Forge, Strike, Cast |
| Ephemeral-DB replay check | `--verify` | Proof, Assay, Dryrun |
| Project scaffolder | `init` | Trailhead, Groundwork, Bedrock |
| Embeddable self-migrate library | *(library API)* | Autopilot, Selfheal, Onward |

## Component Names

| Component | Branded Options |
|---|---|
| Autodetect + DSN resolver (`config`) | Pathfinder, Compass, Locator |
| Shared connection layer (`db`) | Conduit, Junction, Gateway |
| Migration UX layer (goose-style) | Waypost, Logbook, Ledger |
| Codegen engine (parse→catalog→resolve→render) | Forge, Anvil, Crucible |
| Schema catalog model | Register, Codex, Cartulary |
| Static-vs-live schema source | Lens, Mirror, Snapshot |

## Taglines

- Your migrations are the source of truth.
- Migrate. Query. Generate. One schema.
- One schema, zero drift.
- From migration to type-safe Go — one tool.
- Define it once; migrate, query, and generate from it.
- The schema speaks once.
- Type-safe Go, dictated by your migrations.
- Three tools. One source of truth.

## CLI Branding Themes

### Theme 1 — Minimal (recommended default)
```
cairn init
cairn migrate create <name> [-s]
cairn migrate up | down | status | version
cairn generate [--verify]
cairn query [DSN] [-c "SQL"]
```

### Theme 2 — Trail (Cairn-native)
```
cairn trailhead          # init
cairn blaze <name>       # migrate create  (blaze a trail marker)
cairn ascend             # migrate up
cairn descend            # migrate down
cairn survey             # migrate status
cairn chart [--verify]   # generate
cairn scout [DSN]        # query
```

### Theme 3 — Forge
```
forge smelt              # init
forge cast <name>        # migrate create
forge temper             # migrate up
forge anneal             # migrate down
forge gauge              # migrate status
forge strike [--verify]  # generate
forge assay [DSN]        # query
```

## Color Palette Suggestions

| Role | Name | Hex |
|---|---|---|
| Primary | Granite Slate | `#2E3440` |
| Secondary | Tidal Teal | `#1FA8A0` |
| Accent | Marker Amber | `#E0A33E` |
| Warning | Oxide Rust | `#C2503A` |
| Muted | Ash Grey | `#8B919B` |

## Logo Concepts

1. **Stacked stones = stacked migrations:** a 3–4 stone cairn whose stones double as database "disk" cylinders; the top stone glows amber (the latest migration = current truth).
2. **Faceted keystone:** a single stone with three lit facets labeled migrate / query / generate — one source, three outputs.
3. **Ascending-bars monogram "C":** horizontal migration-version bars climbing upward and resolving into a `{ }` code brace — migration → type-safe code.
4. **Strata funnel:** layered sediment lines converging into a downward-pointing data triangle (schema → code), with the top layer highlighted.

### Brand Icon (IconForge)

```
iconforge forge --generate --name cairn \
  --primary "#2E3440" --secondary "#1FA8A0" --accent "#E0A33E" \
  --output build/icons
```
