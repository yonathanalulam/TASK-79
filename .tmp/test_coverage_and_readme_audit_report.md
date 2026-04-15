# Test Coverage Audit

## Scope and Method
- Re-audited after user changes.
- Static inspection evidence: `cmd/server/main.go`, `internal/**/*_test.go`, `internal/testutil/testapp.go`, `run_tests.sh`, `README.md`.
- Execution evidence (user requested rerun): `bash ./run_tests.sh` completed successfully; terminal log shows `test-runner-1 exited with code 0` and `==> All tests passed.` in `C:\Users\yonim\.local\share\opencode\tool-output\tool_d935b47200013g11fhpNbTZDhW:796-811`.

## Backend Endpoint Inventory

Source of truth: `cmd/server/main.go:103-222`

- Total endpoints: **88**
- Public: 2
- Authenticated HTML: 21
- API (`/api`): 65

## API Test Mapping Table

Legend:
- `Type`: `true no-mock HTTP` / `HTTP with mocking` / `unit-only / indirect`.
- Primary no-mock evidence for endpoint coverage: `internal/endpointcoverage/endpoint_test.go` (`TestEndpointCoverage`, `TestLogout`) with full app bootstrap from `internal/testutil/testapp.go` (`MustApp`, `AuthClient`).

| Endpoint | Covered | Type | Test files | Evidence |
|---|---|---|---|---|
| `GET /login` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /login` |
| `POST /login` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /login` |
| `POST /logout` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestLogout` |
| `GET /` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET / (dashboard)` |
| `GET /catalog` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /catalog` |
| `GET /catalog/new` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /catalog/new` |
| `GET /catalog/import` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /catalog/import` |
| `GET /catalog/:id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /catalog/:id` |
| `GET /catalog/:id/edit` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /catalog/:id/edit` |
| `GET /cart` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /cart` |
| `GET /cart/:id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /cart/:id` |
| `GET /cart/:id/merge-modal` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /cart/:id/merge-modal` |
| `GET /cart/:id/add-item` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /cart/:id/add-item` |
| `GET /orders` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /orders` |
| `GET /orders/:id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /orders/:id` |
| `GET /orders/:id/split-modal` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /orders/:id/split-modal` |
| `GET /notifications` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /notifications` |
| `GET /alerts` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /alerts` |
| `GET /alerts/:id/close-modal` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /alerts/:id/close-modal` |
| `GET /metrics` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /metrics` |
| `GET /metrics/:id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /metrics/:id` |
| `GET /metrics/new` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /metrics/new` |
| `GET /audit` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /audit` |
| `GET /api/me` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/me` |
| `GET /api/dashboard/summary` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/dashboard/summary` |
| `GET /api/catalog/brands` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/catalog/brands` |
| `GET /api/catalog/series` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/catalog/series` |
| `GET /api/catalog/models` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/catalog/models` |
| `GET /api/catalog/models/:id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/catalog/models/:id` |
| `POST /api/catalog/models` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/catalog/models` |
| `PUT /api/catalog/models/:id/draft` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `PUT /api/catalog/models/:id/draft` |
| `POST /api/catalog/models/:id/draft` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/catalog/models/:id/draft` |
| `POST /api/catalog/models/:id/publish` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/catalog/models/:id/publish` |
| `POST /api/catalog/models/:id/unpublish` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/catalog/models/:id/unpublish` |
| `POST /api/catalog/models/:id/media` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/catalog/models/:id/media (no file)` |
| `GET /api/catalog/export.csv` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/catalog/export.csv` |
| `POST /api/catalog/imports` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/catalog/imports (no file)` |
| `GET /api/catalog/imports/:job_id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/catalog/imports/:job_id` |
| `POST /api/catalog/imports/:job_id/commit` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/catalog/imports/:job_id/commit` |
| `GET /api/carts` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/carts` |
| `POST /api/carts` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/carts` |
| `GET /api/carts/:id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/carts/:id` |
| `POST /api/carts/:id/items` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/carts/:id/items` |
| `PUT /api/carts/:id/items/:item_id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `PUT /api/carts/:id/items/:item_id` |
| `DELETE /api/carts/:id/items/:item_id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `DELETE /api/carts/:id/items/:item_id` |
| `POST /api/carts/:id/merge` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/carts/:id/merge` |
| `POST /api/carts/:id/revalidate` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/carts/:id/revalidate` |
| `POST /api/carts/:id/checkout` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/carts/:id/checkout` |
| `GET /api/orders` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/orders` |
| `GET /api/orders/:id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/orders/:id` |
| `POST /api/orders/:id/notes` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtests `POST /api/orders/:id/notes (empty)` and `(valid)` |
| `POST /api/orders/:id/payment-recorded` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/orders/:id/payment-recorded` |
| `POST /api/orders/:id/transition` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/orders/:id/transition` |
| `POST /api/orders/:id/split` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/orders/:id/split` |
| `GET /api/orders/:id/timeline` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/orders/:id/timeline` |
| `GET /api/notifications` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/notifications` |
| `POST /api/notifications/:id/read` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/notifications/:id/read` |
| `POST /api/notifications/bulk-read` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/notifications/bulk-read` |
| `GET /api/announcements` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/announcements` |
| `POST /api/announcements/:id/read` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/announcements/:id/read` |
| `GET /api/notification-preferences` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/notification-preferences` |
| `PUT /api/notification-preferences` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `PUT /api/notification-preferences` |
| `POST /api/notification-preferences` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/notification-preferences` |
| `GET /api/export-queue` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/export-queue` |
| `GET /api/alerts` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/alerts` |
| `POST /api/alerts/:id/claim` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/alerts/:id/claim` |
| `POST /api/alerts/:id/process` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/alerts/:id/process` |
| `POST /api/alerts/:id/close` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/alerts/:id/close` |
| `POST /api/alerts/evaluate` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/alerts/evaluate` |
| `GET /api/metrics` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/metrics` |
| `POST /api/metrics` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/metrics (form)` |
| `GET /api/metrics/:id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/metrics/:id` |
| `PUT /api/metrics/:id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `PUT /api/metrics/:id` |
| `GET /api/metrics/:id/versions` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/metrics/:id/versions` |
| `GET /api/metrics/:id/dimensions` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/metrics/:id/dimensions` |
| `POST /api/metrics/:id/dimensions` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/metrics/:id/dimensions` |
| `DELETE /api/metrics/:id/dimensions/:dim_id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `DELETE /api/metrics/:id/dimensions/:dim_id` |
| `GET /api/metrics/:id/filters` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/metrics/:id/filters` |
| `POST /api/metrics/:id/filters` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/metrics/:id/filters` |
| `DELETE /api/metrics/:id/filters/:filter_id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `DELETE /api/metrics/:id/filters/:filter_id` |
| `GET /api/metrics/:id/dependencies` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/metrics/:id/dependencies` |
| `POST /api/metrics/:id/dependencies` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/metrics/:id/dependencies` |
| `DELETE /api/metrics/:id/dependencies/:dep_id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `DELETE /api/metrics/:id/dependencies/:dep_id` |
| `POST /api/metrics/:id/impact-analysis` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/metrics/:id/impact-analysis` |
| `POST /api/metrics/:id/activate` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `POST /api/metrics/:id/activate` |
| `GET /api/metrics/:id/lineage` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/metrics/:id/lineage` |
| `GET /api/audit` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/audit` |
| `GET /api/audit/:entity_type/:entity_id` | yes | true no-mock HTTP | `internal/endpointcoverage/endpoint_test.go` | `TestEndpointCoverage` subtest `GET /api/audit/:entity_type/:entity_id` |

## API Test Classification

1. **True No-Mock HTTP**
   - `internal/endpointcoverage/endpoint_test.go`
     - `TestRouteInventory`
     - `TestEndpointCoverage`
     - `TestLogout`
     - `TestUnauthenticatedAccess`
     - `TestCSRFEnforcement`
   - Evidence of real stack bootstrapping: `internal/testutil/testapp.go`:
     - real DB/migrations/seeding (`MustApp`)
     - real services/handlers wiring (same modules as production)
     - real router + auth/csrf middleware chain
     - real HTTP server (`httptest.NewServer`)

2. **HTTP with Mocking / Bypass**
   - `internal/http/handlers/handlers_test.go`:
     - synthetic context injection via `injectUser`.
     - custom test handlers (`timelineAuthHandler`, local `detailAuthHandler`) bypass production handler functions for some route checks.
   - `internal/http/middleware/csrf_test.go`:
     - synthetic non-production routes (`/page`, `/mutate`, `/remove`).

3. **Non-HTTP (unit/service/repository integration without HTTP transport)**
   - `internal/alerts/*_test.go`, `internal/orders/*_test.go`, `internal/metrics/*_test.go`, `internal/notifications/*_test.go`, `internal/imports/integration_test.go`, `internal/rbac/rbac_test.go`, `internal/crypto/crypto_test.go`, `internal/masking/mask_test.go`, `internal/http/views/templ/render_test.go`, etc.

## Mock Detection

- No framework-level mocks detected in Go tests (`jest.mock`/`vi.mock`/`sinon.stub`/gomock/testify-mock absent).
- Bypass/mocking patterns still present in legacy tests:
  - `internal/http/handlers/handlers_test.go`:
    - `injectUser` sets user/perms/roles/session directly in context.
    - `timelineAuthHandler` and local `detailAuthHandler` avoid calling real endpoint handlers in those specific tests.
- No bypass detected in the new endpoint coverage suite (`internal/endpointcoverage/endpoint_test.go`).

## Coverage Summary

- Total endpoints: **88** (`cmd/server/main.go:103-222`)
- Endpoints with HTTP tests: **88/88**
- Endpoints with true no-mock HTTP tests: **88/88**
- HTTP coverage: **100.00%**
- True API coverage: **100.00%**

## Unit Test Summary

- Test files present across handlers, middleware, services, repository/domain modules, endpoint coverage suite, and utility/domain packages.
- Modules covered:
  - controllers/handlers: now broad via no-mock HTTP (`internal/endpointcoverage/endpoint_test.go`)
  - services: alerts/orders/metrics/notifications/imports integration and unit tests
  - repositories: limited direct tests (`internal/orders/repository_test.go` is mostly structural)
  - auth/guards/middleware: auth+permission exercised through endpoint suite; csrf directly tested

Important modules not directly tested in depth (still notable):
- `internal/scheduler/scheduler.go` (no tests)
- `internal/catalog/repository.go` (no dedicated repository-level tests)
- `internal/cart/repository.go` (no dedicated repository-level tests)

## API Observability Check

- Endpoint/method clarity: **strong** (central endpoint list + route inventory assertion).
- Request input clarity: **strong** (table-driven cases include query/path/form/json per endpoint in `TestEndpointCoverage`).
- Response clarity: **moderate-strong**:
  - all cases assert status;
  - many assert body marker (`"ok":true`, known text);
  - some redirect-only checks are status-focused, with limited payload contract assertions.

## Tests Check

- Success paths: broad, including all 88 endpoints.
- Failure/edge/validation/auth checks: present (`TestUnauthenticatedAccess`, `TestCSRFEnforcement`, negative endpoint cases like empty note/no-file upload).
- Integration boundaries: improved materially by full app bootstrap and real middleware/handlers/services.
- `run_tests.sh`: Docker-based orchestration; no local dependency requirement for the test run path.
- Rerun status: **PASS** (`==> All tests passed.`).

## End-to-End Expectations

- Repo is fullstack-style (server-rendered web + API).
- Browser-level FE automation is still absent; however, strong real HTTP route coverage + service integration gives substantial compensation for backend/API correctness.

## Test Coverage Score (0-100)

**93 / 100**

## Score Rationale

- + Full endpoint coverage and full true no-mock API coverage (88/88).
- + Real stack execution with auth/csrf and DB-backed services.
- + Route inventory drift guard present.
- - Some assertions remain shallow (status + substring instead of deep payload contracts for many endpoints).
- - Legacy HTTP tests with bypass patterns still exist (non-blocking now, but still present).
- - Some non-HTTP modules (scheduler/repository depth) remain lightly tested.

## Key Gaps

- Deep contract assertions could be expanded for selected critical mutation endpoints.
- Legacy bypass-style handler tests could be reduced or clearly scoped to helper behavior only.
- Scheduler path lacks dedicated tests.

## Confidence & Assumptions

- Confidence: **High**.
- Assumptions:
  - Production endpoint source remains `cmd/server/main.go`.
  - 88 endpoints in route table are the required audit scope.

**Test Coverage Verdict: PASS**

---

# README Audit

## Project Type Detection

- Explicitly declared at top: `Project Type: fullstack` (`README.md:3`).
- Inferred architecture matches declaration (Go backend + web UI + API).

## README Location

- Present at required location: `README.md`.

## Hard Gate Checks

### Formatting
- Clean markdown and readable structure: **PASS**.

### Startup Instructions
- Required literal command present: `docker-compose up` (`README.md:50`): **PASS**.

### Access Method
- URL/port provided (`README.md:61`, `README.md:65`): **PASS**.

### Verification Method
- UI verification steps included (`README.md:82-88`).
- API verification with expected indicators included (`README.md:89-113`).
- **PASS**.

### Environment Rules (Docker-contained)
- No runtime package install/manual DB setup instructions in README.
- Previous non-Docker manual setup section removed.
- **PASS**.

### Demo Credentials (Auth conditional)
- Auth exists and README provides username/password for all roles (`README.md:67-75`).
- **PASS**.

## Engineering Quality

- Tech stack and architecture clarity: strong.
- Security/roles/workflows coverage: strong.
- Testing instructions: includes Docker-based test runner (`README.md:188-200`).
- Presentation quality: strong and operationally aligned.

## High Priority Issues

- None.

## Medium Priority Issues

- API verification command for login token extraction is complex and may be shell-sensitive across environments (`README.md:93-95`).

## Low Priority Issues

- Could add a shorter optional "quick verify" variant for easier copy-paste.

## Hard Gate Failures

- None.

## README Verdict

**PASS**

**README Final Verdict: PASS**
