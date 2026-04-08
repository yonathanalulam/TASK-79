# Fix Check Report - audit_report_1 Issues (Round 1)

## 1. Scope
- Source of issues reviewed: `.tmp/audit_report_1.md` (current contents list 2 open issues from prior inspection).
- Method: static-only verification (no runtime execution, no tests run).

## 2. Issue-by-Issue Fix Verification

### Issue 1: Order timeline endpoint bypasses object-level authorization (High)
- Previous finding: `OrderTimelineAPI` returned timeline by `id` without object authorization.
- Current status: **Fixed**
- Evidence:
  - `internal/http/handlers/handlers.go:1092` now calls `h.loadAndAuthorizeOrder(c)`.
  - `internal/http/handlers/handlers.go:1093`-`internal/http/handlers/handlers.go:1098` enforces authorization before `ListHistory`.
  - Guard logic already uses scoped/global checks via `enforceOrderAccess` (`internal/http/handlers/handlers.go:216`-`internal/http/handlers/handlers.go:231`).
- Conclusion: the previously reported IDOR-style timeline access gap is remediated in handler code.

### Issue 2: Missing tests for unauthorized `/api/orders/:id/timeline` access (Medium)
- Previous finding: no tests covered timeline object-authorization behavior.
- Current status: **Fixed (static evidence)**
- Evidence:
  - New timeline authorization test block present in `internal/http/handlers/handlers_test.go:207` onward.
  - Unauthorized access test exists: `internal/http/handlers/handlers_test.go:253`-`internal/http/handlers/handlers_test.go:281` (expects 403 for non-owner).
  - Authorized/global-scope tests exist: `internal/http/handlers/handlers_test.go:283`, `internal/http/handlers/handlers_test.go:307`, `internal/http/handlers/handlers_test.go:330`.
  - Consistency test across timeline/detail authorization exists: `internal/http/handlers/handlers_test.go:354`-`internal/http/handlers/handlers_test.go:435`.
- Conclusion: explicit coverage for timeline authorization behavior is now present.

## 3. Overall Fix-Check Verdict
- Result for issues previously listed in `.tmp/audit_report_1.md`: **All checked issues are fixed by static evidence.**
- Remaining boundary note: tests are present in code but were not executed in this static-only check.
