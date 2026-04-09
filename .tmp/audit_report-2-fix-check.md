# Fix Check Report - Round 2 (`audit_report_2_fix-check_round1.md`)

## Verdict
- Overall: **All previously listed issues are fixed**
- Fixed: **4 / 4**
- Partially fixed: **0 / 4**
- Still open: **0 / 4**

## Scope Boundary
- Static-only verification of the four issues tracked in `.tmp/audit_report_2_fix-check_round1.md`.
- No runtime execution, no Docker, no tests executed.

## Issue Recheck Results

### 1) Metric semantic-layer delivery incomplete (High)
- Previous status: Not Fixed
- Current status: **Fixed**
- Evidence:
  - Metric API now includes richer management endpoints for dimensions/filters/dependencies and update flow: `cmd/server/main.go:204`, `cmd/server/main.go:207`, `cmd/server/main.go:210`, `cmd/server/main.go:213`
  - `CreateMetricAPI` now accepts and forwards `is_derived`, `window_calculation`, `depends_on_metrics`, `filters`, and `dimensions`: `internal/http/handlers/handlers.go:1409`, `internal/http/handlers/handlers.go:1410`, `internal/http/handlers/handlers.go:1441`, `internal/http/handlers/handlers.go:1443`
  - Service persists these fields transactionally: `internal/metrics/service.go:188`, `internal/metrics/service.go:194`, `internal/metrics/service.go:205`, `internal/metrics/service.go:212`

### 2) Alert lifecycle mutations ignored DB/audit errors (Medium)
- Previous status: Not Fixed
- Current status: **Fixed**
- Evidence:
  - Claim flow now checks update/event/audit errors: `internal/alerts/service.go:107`, `internal/alerts/service.go:110`, `internal/alerts/service.go:114`
  - Process flow now checks update/event/audit errors: `internal/alerts/service.go:144`, `internal/alerts/service.go:147`, `internal/alerts/service.go:151`
  - Close flow now checks update/event/audit errors: `internal/alerts/service.go:185`, `internal/alerts/service.go:189`, `internal/alerts/service.go:194`

### 3) Notification mutation paths ignored critical write errors (Medium)
- Previous status: Not Fixed
- Current status: **Fixed**
- Evidence:
  - Preference update path now checks delete/insert/audit errors: `internal/notifications/service.go:196`, `internal/notifications/service.go:200`, `internal/notifications/service.go:206`
  - Export retry bookkeeping now checks both queue-update and attempt-log insert errors: `internal/notifications/service.go:273`, `internal/notifications/service.go:277`

### 4) High-risk workflow test coverage shallow (Medium)
- Previous status: Not Fixed
- Current status: **Fixed**
- Evidence:
  - Split integrity integration coverage added, including cross-order line rejection: `internal/orders/integration_test.go:11`, `internal/orders/integration_test.go:215`, `internal/orders/integration_test.go:255`
  - Import commit workflow/invariants covered by integration tests: `internal/imports/integration_test.go:40`, `internal/imports/integration_test.go:80`
  - Announcement read persistence and export artifact workflow covered: `internal/notifications/integration_test.go:222`, `internal/notifications/integration_test.go:58`, `internal/notifications/integration_test.go:177`
  - Metric semantic-layer persistence coverage added (derived/window/dimensions/filters/dependencies): `internal/metrics/integration_test.go:11`, `internal/metrics/integration_test.go:41`, `internal/metrics/integration_test.go:110`

## Final Note
- Based on static evidence, the four tracked defects from round 1 are now resolved.
