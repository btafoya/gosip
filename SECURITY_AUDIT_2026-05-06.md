# GoSIP Security & Code Quality Audit

**Date:** 2026-05-06
**Scope:** Full codebase review — backend (Go), database layer, SIP package, frontend (Vue/TS)
**Methodology:** Parallel agent review across API layer, DB layer, SIP package, core infrastructure, and frontend

---

## Severity Legend

- **Critical** — Exploitable security vulnerability, data loss, or guaranteed crash/panic
- **High** — Significant bug, performance issue, or security weakness under normal operation
- **Medium** — Correctness issue, missing validation, or code smell with real impact
- **Low** — Minor inefficiency, style issue, or edge-case concern

---

## Critical (7 issues)

### C1. Stored TwiML injection — call redirection / toll fraud
- **File:** `internal/api/webhooks.go`
- **Lines:** 310-332, 394, 418-419
- **Issue:** `errorTwiML`, `rejectTwiML`, `voicemailTwiML`, and `executeAction` build XML by string concatenation without escaping caller-controlled values (`message`, `reason`, `greeting`, `device.Username`, `did.Number`, `data.Number`). Only `smsTwiML` calls `escapeXML`. An attacker who creates a device with username `</Sip><Redirect>tel:+19005551212</Redirect><Sip>` or crafts a DID number with XML metacharacters will have that payload stored in the DB and later injected into TwiML responses sent to Twilio, redirecting calls to arbitrary numbers.
- **Fix:** Apply `escapeXML()` (or `html.EscapeString`) to every user-controlled value inserted into TwiML.
- **Status:** **Fixed** (2026-05-06)

### C2. Most Twilio webhooks skip signature validation — trivial spoofing
- **File:** `internal/api/webhooks.go`
- **Lines:** 83 (`VoiceStatus`), 109 (`VoicemailRecording`), 157 (`VoicemailTranscription`), 245 (`SMSStatus`)
- **Issue:** Only `VoiceIncoming:38` calls `validateSignature()`. The other four webhook endpoints accept any POST request. An attacker can inject fake call status updates, create fake voicemail records, trigger fake transcription completions, and inject fake SMS messages.
- **Fix:** Add `if !h.validateSignature(r) { return }` guard to every webhook handler.
- **Status:** **Fixed** (2026-05-06)

### C3. Fire-and-forget goroutines crash the process on panic
- **Files:** `internal/api/webhooks.go:140,145,150,239`, `internal/api/middleware.go:181`, `internal/twilio/queue.go:51`, `pkg/sip/server.go` background workers
- **Issue:** `chi/middleware.Recoverer` only catches panics in the request goroutine. Any panic in spawned goroutines (async Twilio SMS send, transcription requests, voicemail notifications, MWI updates, session refresh) kills the entire server process. The middleware goroutine is especially dangerous — it fires on every authenticated request that hits the session cache.
- **Fix:** Introduced `safeGo(f func())` helper in `internal/api/safe.go` that wraps each goroutine with `defer recover()` + structured error logging. Applied to all fire-and-forget goroutines across API, twilio queue, and SIP background workers.
- **Status:** **Fixed** (2026-05-06)

### C4. RestoreBackup race condition + permanent DB corruption
- **File:** `internal/db/db.go`
- **Lines:** 495-521
- **Issue:** `RestoreBackup` closes `db.conn` and replaces it without any mutex or synchronization. If another goroutine executes a query concurrently, it will panic or fail unpredictably. Worse, if `copyFile` or `conn.PingContext` fails after the connection is closed, the error is returned but `db.conn` is never reopened, leaving the `DB` object permanently unusable for the process lifetime.
- **Fix:** Protect `db.conn` and all repository pointers with a `sync.RWMutex`. On restore failure, reopen the original database or fail fast with a clear fatal error.
- **Status:** **Fixed** (2026-05-06)

### C5. Migration 003 down destroys provisioning data permanently
- **File:** `internal/db/migrations/003_provisioning.down.sql`
- **Lines:** 22-36
- **Issue:** The down migration recreates `devices_backup` with only the original 8 columns (no `mac_address`, `vendor`, `model`, `firmware_version`, `provisioning_status`, `last_config_fetch`, `last_registration`, or `config_template`). The `INSERT` also selects only those 8 columns. Rolling back migration 003 permanently drops all provisioning data.
- **Fix:** Recreate `devices_backup` with the full schema as it existed after `003.up`, or at minimum include all columns in the `SELECT`.
- **Status:** **Fixed** (2026-05-06)

### C6. BYE / CANCEL / REFER call teardown spoofing
- **File:** `pkg/sip/handlers.go`
- **Lines:** 173-214 (`handleBye`), 218-235 (`handleCancel`), 238-246 (`handleRefer`)
- **Issue:** These handlers accept BYE/CANCEL/REFER for any `Call-ID` and terminate or transfer the session without validating the `From`/`To` tags against the dialog state, or verifying the request originates from a registered device. An attacker who knows (or guesses) a `Call-ID` can tear down or transfer any active call.
- **Fix:** Verify the request originates from a registered device or Twilio before acting on mid-call requests. Validate dialog tags.
- **Status:** **Fixed** (2026-05-06)

### C7. Frontend open redirect after login
- **File:** `frontend/src/views/LoginView.vue`
- **Lines:** 13-18
- **Issue:** After successful login, the app redirects to `route.query.redirect` without validating that it is a same-origin relative path. An attacker can craft a link like `/login?redirect=https://evil.com` to phish users post-authentication.
- **Fix:** Added `isInternalPath()` helper in `frontend/src/utils/validation.ts` that rejects non-strings, paths not starting with `/`, protocol-relative URLs (`//`), and absolute URLs with schemes.
- **Status:** **Fixed** (2026-05-06)

---

## High (18 issues)

### H1. Unbounded goroutine explosion — DoS vector
- **File:** `internal/twilio/queue.go`
- **Lines:** 47-52
- **Issue:** `MessageQueue.Enqueue` spawns `go q.processMessage(msg)` for each additional message when the queue (size 1000) is full. Under load this exhausts memory and goroutine limits.
- **Fix:** Use a bounded worker pool or drop messages with an error.
- **Status:** **Fixed** (2026-05-06)

### H2. ZRTP cache unbounded memory growth
- **File:** `pkg/sip/zrtp.go`
- **Lines:** 410-450, 452-468
- **Issue:** `cacheSession` adds entries to `m.cache.entries` forever. `getCacheEntry` checks expiry but never deletes expired entries. Long-running server leaks memory.
- **Fix:** Expire entries during lookup or run a periodic cleanup goroutine.
- **Status:** **Fixed** (2026-05-06)

### H3. SIP rejects all external incoming calls
- **File:** `pkg/sip/handlers.go`
- **Lines:** 149-163
- **Issue:** All unauthenticated incoming INVITEs return `486 Busy Here`. The TODO comment says "Validate request is from Twilio and route to appropriate device" but this was unimplemented. The system could not receive calls in production.
- **Fix:** Implemented incoming INVITE routing: extract called number, look up DID, evaluate enabled routes (default/time conditions), forward to registered devices (ring action), redirect to voicemail (voicemail action), or reject (reject action). Added basic Twilio source logging.
- **Status:** **Fixed** (2026-05-06)

### H4. SMTP port storage corrupted
- **File:** `internal/api/system.go`
- **Line:** 218
- **Issue:** `h.deps.DB.Config.Set(r.Context(), "smtp_port", string(rune(req.SMTPPort)))` converts the integer to a Unicode rune string, not a decimal string. For example, 587 becomes a single glyph (U+024B), not `"587"`. SMTP configuration is permanently broken after saving settings.
- **Fix:** Use `strconv.Itoa(req.SMTPPort)`.
- **Status:** **Fixed** (2026-05-06)

### H5. Negative offset panics voicemail list
- **File:** `internal/api/voicemails.go`
- **Lines:** 70-78
- **Issue:** `voicemails[offset:end]` slices with no lower-bound check. `offset` from `strconv.Atoi` can be negative. `offset=-100` causes an index-out-of-range panic.
- **Fix:** Validate `offset >= 0` and `limit > 0` on all list endpoints.
- **Status:** **Fixed** (2026-05-06)

### H6. Panic on short provisioning token in QR code
- **File:** `internal/api/provisioning.go`
- **Line:** 572
- **Issue:** `w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"provision-%s.png\"", tokenStr[:8]))` panics if the DB returns a token shorter than 8 characters.
- **Fix:** Check `len(tokenStr) >= 8` before slicing.
- **Status:** **Fixed** (2026-05-06)

### H7. Login rate-limit bypass via X-Forwarded-For
- **File:** `internal/api/routes.go`
- **Line:** 18
- **Issue:** `middleware.RealIP` trusts `X-Forwarded-For` / `X-Real-IP` without verifying the source is a trusted proxy. An attacker can spoof any IP to evade their own rate-limit record or lock out another IP address.
- **Fix:** Replaced `chi/middleware.RealIP` with custom `TrustedProxyIP` middleware that only trusts `X-Forwarded-For` when the immediate connection originates from loopback. Uses direct `r.RemoteAddr` otherwise.
- **Status:** **Fixed** (2026-05-06)

### H8. Password change does not invalidate existing sessions
- **File:** `internal/api/auth.go`
- **Lines:** 165-207
- **Issue:** `ChangePassword` updates the password hash but does not kill existing sessions. A stolen session token remains valid after the user changes their password.
- **Fix:** Delete all sessions for the user on password change.
- **Status:** **Fixed** (2026-05-06)

### H9. MOH file path traversal + no size limit
- **File:** `pkg/sip/moh.go`
- **Lines:** 45-46, 182-204
- **Issue:** `audioPath` has no validation. If user-controlled via API, arbitrary file read is possible. `io.ReadAll(file)` loads the entire file into memory with no size cap — OOM possible with a symlink to `/dev/zero`.
- **Fix:** `SetAudioPath` validates resolved absolute path is under configured base directory using `filepath.Abs` + `strings.HasPrefix`. `loadAudioFile` uses `io.LimitReader(file, maxMOHSize)` with 50MB cap.
- **Status:** **Fixed** (2026-05-06)

### H10. Queue Stop double-close panic
- **File:** `internal/twilio/queue.go`
- **Lines:** 78-85
- **Issue:** `close(q.stopChan)` has no guard against a second call. Calling `Stop()` twice panics.
- **Fix:** Use `sync.Once` or set `stopChan = nil` after close.
- **Status:** **Fixed** (2026-05-06)

### H11. Backup filename collision
- **File:** `internal/db/db.go`
- **Line:** 296
- **Issue:** `CreateBackup` uses `time.Now().Format("20060102_150405")` (second precision). Two backups triggered in the same second overwrite each other silently.
- **Fix:** Append milliseconds (`20060102_150405.000`) to backup filename. Updated `validateFilename` regex to accept millisecond suffix.
- **Status:** **Fixed** (2026-05-06)

### H12. Timezone mismatch in message statistics
- **File:** `internal/db/messages.go`
- **Lines:** 365-375
- **Issue:** `GetStats` uses SQLite `date('now')` and `datetime('now', '-7 days')`, which operate in UTC. However, `created_at` is stored as local time from `time.Now()`. For any non-UTC server, the "today", "this_week", and "this_month" counts will be wrong by the timezone offset.
- **Fix:** Use `datetime('now', 'localtime')` in SQLite queries to match local-time `created_at` storage.
- **Status:** **Fixed** (2026-05-06)

### H13. Missing ON DELETE cascade breaks cleanup
- **File:** `internal/db/migrations/001_initial_schema.up.sql`
- **Issue:** `messages.did_id`, `cdrs.did_id`, `cdrs.device_id`, and `voicemails.cdr_id` / `voicemails.user_id` reference parent tables without `ON DELETE CASCADE` or `ON DELETE SET NULL`. With foreign keys enabled, deleting a DID, device, CDR, or user that has child records causes a hard constraint error instead of a clean cascade.
- **Fix:** Added `ON DELETE CASCADE` to `devices.user_id` and `ON DELETE SET NULL` to `cdrs.did_id`, `cdrs.device_id`, `messages.did_id`, `voicemails.cdr_id`, `voicemails.user_id`.
- **Status:** **Fixed** (2026-05-06)

### H14. Missing rows.Err() check
- **File:** `internal/db/sessions.go`
- **Lines:** 150-191
- **Issue:** `ListByUserID` returns `sessions, nil` after `rows.Next()` finishes. If `rows.Next()` stopped because of a database error, that error is silently swallowed.
- **Fix:** Return `sessions, rows.Err()`.
- **Status:** **Fixed** (2026-05-06)

### H15. Partial-update booleans silently disable features
- **Files:** `internal/api/messages.go:430`, `internal/api/routes.go:184`, `internal/api/provisioning.go:407`
- **Issue:** Update handlers use plain `bool` (not `*bool`) in request structs. Omitting the `enabled` field in JSON sets it to `false`, silently disabling auto-replies, routes, or profiles.
- **Fix:** Converted `Enabled bool` to `Enabled *bool` in update request structs for routes, messages, and provisioning. Only assign when pointer is non-nil. Replicated `*bool` pattern from `dids.go`.
- **Status:** **Fixed** (2026-05-06)

### H16. JWT stored in localStorage — XSS vector
- **File:** `frontend/src/api/client.ts`
- **Line:** 16
- **Issue:** The JWT is read from `localStorage`. If any XSS vector were introduced (or a malicious browser extension is present), the token is trivially exfiltrated.
- **Fix:** Removed all `localStorage` token usage. Rely on existing `HttpOnly`, `Secure`, `SameSite=Strict` session cookie. Removed request interceptor that injected Bearer token. Backend cookie hardening applied in M5.
- **Status:** **Fixed** (2026-05-06)

### H17. Audio element memory leak + race condition
- **File:** `frontend/src/views/VoicemailsView.vue`
- **Lines:** 122-151
- **Issue:** `playVoicemail()` overwrites `audioRef` without removing the previous element's `onended` listener. If a user clicks a different voicemail while one is playing, the old audio element remains in memory and its `onended` handler will later corrupt state. Additionally, there is no `onUnmounted` cleanup.
- **Fix:** Before creating new `Audio`, set `audioRef.value.onended = null`. Added `onUnmounted` cleanup that pauses and nulls `audioRef`. Added `.catch()` to `audio.play()`.
- **Status:** **Fixed** (2026-05-06)

### H18. Frontend settings form clears stored passwords
- **File:** `frontend/src/views/SettingsView.vue`
- **Lines:** 81-100, 166-180
- **Issue:** `loadSystemSettings` always populates password fields with empty strings. `saveSystemSettings` then sends those empty strings to `PUT /system/config`.
- **Fix:** In `saveSystemSettings`, strip empty password fields (`twilio_auth_token`, `smtp_password`, `gotify_token`) from the payload before sending.
- **Status:** **Fixed** (2026-05-06)

---

## Medium (13 issues)

### M1. Nil context passed to DB layer
- **File:** `internal/api/webhooks.go`
- **Lines:** 319, 394
- **Issue:** `h.deps.DB.Config.Get(nil, "voicemail_greeting")` and `h.deps.DB.Devices.GetByID(nil, deviceID)` pass `nil` context. This can cause panics or undefined behavior if the DB implementation relies on context deadlines or cancellation.
- **Fix:** Replaced `nil` context with `r.Context()` in `voicemailTwiML` and `executeAction`. Updated function signatures to accept `context.Context`.
- **Status:** **Fixed** (2026-05-06)

### M2. Widespread ignored errors
- **Files:** Many across `internal/api/`
- **Representative examples:**
  - `webhooks.go` 94-103: `CDRs.Update` error ignored — call records inconsistent
  - `webhooks.go` 119: `strconv.ParseInt(didIDStr)` error ignored — `didID` becomes 0, voicemail attached to wrong DID
  - `webhooks.go` 134: `Voicemails.Create` error ignored — voicemail may be lost
  - `webhooks.go` 229: `Messages.Create` error ignored — inbound SMS may be lost
  - `auth.go` 229: `Users.Count` error ignored — pagination total wrong
  - `system.go` 64: `fmt.Sscanf(portStr)` error ignored — invalid port silently accepted
- **Fix:** Systematic error-handling pass. All discarded DB/JSON errors now logged with `slog.Error`. For write failures, return 500 to client. `strconv`/`fmt.Sscanf` errors now return 400 for invalid input.
- **Status:** **Fixed** (2026-05-06)

### M3. Lockout duration uses oldest attempt, not newest
- **File:** `internal/api/auth.go`
- **Lines:** 410-418
- **Issue:** `lockoutEnd := recent[0].Add(config.LoginLockoutDuration)` — `recent[0]` is the oldest failed attempt. Lockout timing is wrong.
- **Fix:** Use `recent[len(recent)-1]` (the most recent attempt).
- **Status:** **Fixed** (2026-05-06)

### M4. 409 Conflict returned for arbitrary DB errors
- **Files:** `internal/api/dids.go:89`, `internal/api/auth.go:286`
- **Issue:** `Create` handlers return HTTP 409 for any DB error, not just unique-constraint violations. Disk-full, locked DB, and other server errors are misreported as client conflicts.
- **Fix:** Added `isUniqueConstraintError(err) bool` helper in `errors.go`. Return 409 only for unique/CHECK constraint violations; 500 for all other DB errors. Applied consistently across dids, auth, devices, and provisioning handlers.
- **Status:** **Fixed** (2026-05-06)

### M5. Cookie Secure flag breaks behind reverse proxy
- **File:** `internal/api/auth.go`
- **Line:** 116
- **Issue:** `Secure: r.TLS != nil` is false when running behind a TLS-terminating reverse proxy. Session cookies are sent without the Secure flag, making them vulnerable to interception on insecure networks.
- **Fix:** Changed `Secure: r.TLS != nil` to `Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"`. Set `SameSite: http.SameSiteStrictMode`.
- **Status:** **Fixed** (2026-05-06)

### M6. Session cache never cleaned in production
- **File:** `internal/api/middleware.go`
- **Lines:** 125-264
- **Issue:** `cleanupExpiredSessions` exists but nothing calls it on a timer. The in-memory cache grows forever.
- **Fix:** Started background `time.Ticker` in `init()` calling `cleanupExpiredSessions()` every 5 minutes.
- **Status:** **Fixed** (2026-05-06)

### M7. Backup directories world-readable
- **File:** `internal/db/db.go`
- **Lines:** 87, 279
- **Issue:** `os.MkdirAll(backupsDir, 0755)` creates backups with world-read permissions. Backups contain password hashes, auth tokens, call records, and message content.
- **Fix:** Changed `os.MkdirAll(..., 0755)` to `0750`.
- **Status:** **Fixed** (2026-05-06)

### M8. Blocklist check is O(n) and loads entire table
- **File:** `internal/db/blocklist.go`
- **Lines:** 98-131
- **Issue:** `IsBlocked` calls `r.List(ctx)` (full table scan) then iterates all entries in Go for every single number check. This becomes a DoS vector as the blocklist grows.
- **Fix:** Replaced full-table scan with targeted DB queries: exact match first, then `LIKE` prefix match, then regex patterns loaded only when needed.
- **Status:** **Fixed** (2026-05-06)

### M9. Schema drift after rolling back migration 005
- **File:** `internal/db/migrations/005_add_linphone_device_type.down.sql`
- **Issue:** The down migration recreates `devices` but only restores `idx_devices_username` and `idx_devices_user_id`. It omits `idx_devices_mac` and `idx_devices_provisioning_status` added in `003.up`.
- **Fix:** Added `idx_devices_mac` and `idx_devices_provisioning_status` to the down migration.
- **Status:** **Fixed** (2026-05-06)

### M10. SIP header injection in MWI NOTIFY
- **File:** `pkg/sip/server.go`
- **Lines:** 424-430
- **Issue:** `SendMWINotify` builds `Contact`, `From`, `To`, and `CSeq` headers with `fmt.Sprintf` using subscription fields without sanitizing SIP special characters (newlines, `<`, `>`). If any of these fields are attacker-controlled, arbitrary SIP headers can be injected.
- **Fix:** Created `sanitizeSIPHeaderValue()` in `pkg/sip/util.go` that strips `\r`, `\n`, `<`, `>`, and control characters. Applied to all subscription fields in `SendMWINotify`.
- **Status:** **Fixed** (2026-05-06)

### M11. Nil pointer risks in MWI handlers
- **File:** `pkg/sip/handlers_mwi.go`
- **Lines:** 129-134, 159-163, 195-202
- **Issue:** `resp.To()`, `req.Via()`, `req.From()` accessed without nil checks. Malformed SIP requests cause nil-pointer panics.
- **Fix:** Added nil guards for `req.From()`, `req.To()`, `req.Via()`, `req.CallID()` in both `handleMWISubscribe` and `handleMWIUnsubscribe`. Return 400 Bad Request if mandatory headers missing.
- **Status:** **Fixed** (2026-05-06)

### M12. SIPTrunkHandler nil pointer + no escaping
- **File:** `internal/api/webhooks.go`
- **Lines:** 515-536
- **Issue:** `url.Parse("sip:" + to)` error is ignored; `toURI.User` may be nil. Also `from` is injected into TwiML without escaping at line 533.
- **Fix:** Check `url.Parse` error; validate `toURI != nil && toURI.User != nil`; escape `from` and `device.Username` with `html.EscapeString` before TwiML.
- **Status:** **Fixed** (2026-05-06)

### M13. Frontend unhandled promise rejections
- **Files:** `frontend/src/views/ProvisioningView.vue:223-225` (clipboard), `frontend/src/views/VoicemailsView.vue:141` (audio play)
- **Issue:** `navigator.clipboard.writeText(text)` and `audioRef.value.play()` return Promises that are never caught. If the clipboard API is blocked or audio playback fails, unhandled rejections are thrown.
- **Fix:** Added `.catch(() => { ... })` to both `navigator.clipboard.writeText` and `audioRef.value.play()`.
- **Status:** **Fixed** (2026-05-06)

---

## Low (5 issues)

### L1. No max-length / format validation on many inputs
- **Files:** `internal/api/auth.go`, `internal/api/devices.go`, `internal/api/dids.go`, `internal/api/system.go`, `internal/api/tls.go`
- **Examples:**
  - `CreateUserRequest.Email` — no email format check
  - `CreateDeviceRequest.Name/Username/Password` — no max length
  - `CreateDIDRequest.Number` — no phone-number format check
  - `TLSConfigRequest.CertFile/KeyFile` — no path traversal check
- **Fix:** Added basic input validators: `isValidEmail` (contains `@`), `isValidPhoneNumber` (only `+`, digits, spaces, dashes, parens), max length checks, path traversal checks (`..`). Return 400 for invalid input.
- **Status:** **Fixed** (2026-05-06)

### L2. Static file server masks API 404s
- **File:** `internal/api/routes.go`
- **Line:** 281
- **Issue:** `r.Handle("/*", http.FileServer(http.Dir("./frontend/dist")))` catch-all means unknown API routes return the SPA HTML instead of a proper 404, confusing API clients.
- **Fix:** Replaced catch-all with explicit handlers: `r.Get("/", ...)` serves `index.html`, `r.Get("/*", ...)` checks if file exists with `os.Stat` before serving, falling back to `index.html` only for missing files (SPA routing) while preserving API 404s.
- **Status:** **Fixed** (2026-05-06)

### L3. Migration sort is lexicographic, not numeric
- **File:** `internal/db/db.go`
- **Lines:** 133-144
- **Issue:** The code relies on `os.ReadDir` lexicographic sorting. Unpadded filenames like `10_` sort before `2_`, potentially applying migrations out of order.
- **Fix:** Parse version numbers via `fmt.Sscanf` and sort numerically with `sort.Slice`.
- **Status:** **Fixed** (2026-05-06)

### L4. Malformed migration filenames break tracking
- **File:** `internal/db/db.go`
- **Line:** 150
- **Issue:** `fmt.Sscanf(filename, "%d_", &version)` return value is ignored. A file named `patch.up.sql` parses as version `0`, causing incorrect migration state.
- **Fix:** Check `fmt.Sscanf` return value; skip malformed filenames with a warning log instead of silently accepting version 0.
- **Status:** **Fixed** (2026-05-06)

### L5. Frontend incorrect multipart Content-Type
- **File:** `frontend/src/api/calls.ts`
- **Lines:** 129-142, 146-159
- **Issue:** Manually setting `headers: { 'Content-Type': 'multipart/form-data' }` on an axios request that sends `FormData` strips the required `boundary` parameter, which can cause the server to fail parsing the upload.
- **Fix:** Removed the manual header from both `uploadMOHAudio` and `validateMOHAudio`. Let axios set the full multipart header with boundary automatically.
- **Status:** **Fixed** (2026-05-06)

---

## Summary

| Severity | Count |
|----------|-------|
| Critical | 7 |
| High | 18 |
| Medium | 13 |
| Low | 5 |
| **Total** | **43** |

**All 43 issues resolved as of 2026-05-06.**

---

## Verification

- `go test ./...` — all packages pass
- `go build ./...` — clean build
- Frontend `vite build` — successful
