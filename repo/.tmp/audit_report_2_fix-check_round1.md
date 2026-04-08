# Fix Check Report - Based on `.tmp/audit_report_2.md`

## Verdict
- Overall: **Not fully fixed**
- Fixed: **0 / 4**
- Partially fixed: **0 / 4**
- Still open: **4 / 4**

## Scope Boundary
- Static-only recheck of the issues listed in `Section 5` of `.tmp/audit_report_2.md`.
- No runtime execution, no tests run, no Docker/startup.

## Issue-by-Issue Recheck

### 1) Metric semantic-layer delivery incomplete (High)
- Previous finding: missing management flows for dimensions/filters/window/derived semantics; `is_derived` dropped on create.
- Current status: **Not Fixed**
- Evidence:
  - Metric routes are still limited to list/create/impact-analysis/activate/lineage: `cmd/server/main.go:201`
  - `CreateMetricAPI` still parses `IsDerived` from JSON but does not pass it into `metrics.CreateParams`: `internal/http/handlers/handlers.go:1418`, `internal/http/handlers/handlers.go:1436`
  - Schema still has richer objects (dimensions/filters) without corresponding management endpoints: `internal/db/migrations/001_initial_schema.up.sql:458`, `internal/db/migrations/001_initial_schema.up.sql:466`

### 2) Alert lifecycle mutations ignore DB/audit errors (Medium)
- Previous finding: `claim/process/close` paths call `Exec` and `LogTx` without checking errors.
- Current status: **Not Fixed**
- Evidence:
  - Unchecked update/insert in claim flow: `internal/alerts/service.go:106`, `internal/alerts/service.go:107`
  - Unchecked update/insert in process flow: `internal/alerts/service.go:137`, `internal/alerts/service.go:138`
  - Unchecked update/insert/audit in close flow: `internal/alerts/service.go:172`, `internal/alerts/service.go:173`, `internal/alerts/service.go:176`

### 3) Notification mutation paths ignore critical write errors (Medium)
- Previous finding: preference updates and export retry bookkeeping use unchecked writes.
- Current status: **Not Fixed**
- Evidence:
  - Preferences path still uses unchecked `Exec` and unchecked `LogTx`: `internal/notifications/service.go:194`, `internal/notifications/service.go:196`, `internal/notifications/service.go:200`
  - Export retry bookkeeping still uses unchecked DB writes: `internal/notifications/service.go:265`, `internal/notifications/service.go:267`

### 4) High-risk workflow test coverage remains shallow (Medium)
- Previous finding: insufficient DB-backed coverage for split/import/announcement/export workflows.
- Current status: **Not Fixed**
- Evidence:
  - Split tests are still guard-only (empty line list), not DB-backed eligibility/ownership tests: `internal/orders/audit_test.go:15`, `internal/orders/audit_test.go:24`
  - No `internal/imports/*_test.go` test file found in repository scope
  - Announcement coverage is still template-render level, not persistence/API behavior: `internal/http/views/templ/render_test.go:275`
  - No tests mapped for export artifact persistence path in `ProcessExportRetries`

## Final Check Summary
- The previously reported issues from `.tmp/audit_report_2.md` remain open.
- Notable improvements exist elsewhere in the codebase (outside this four-issue fix-check), but they do **not** close these specific four findings.
