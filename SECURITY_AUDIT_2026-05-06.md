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
- **Files:** `internal/api/webhooks.go:140,145,150,239`, `internal/api/middleware.go:181`, `internal/twilio/queue.go:51`
- **Issue:** `chi/middleware.Recoverer` only catches panics in the request goroutine. Any panic in spawned goroutines (async Twilio SMS send, transcription requests, voicemail notifications, MWI updates, session refresh) kills the entire server process. The middleware goroutine is especially dangerous — it fires on every authenticated request that hits the session cache.
- **Fix:** Wrap each goroutine body in `defer func() { recover() }()` or use a bounded worker pool.

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
- **Fix:** Validate the redirect against an allowlist of internal paths or ensure it starts with `/` and not `//`.

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
- **Issue:** All unauthenticated incoming INVITEs return `486 Busy Here`. The TODO comment says "Validate request is from Twilio and route to appropriate device" but this is unimplemented. The system cannot receive calls in production.
- **Fix:** Implement incoming call routing from Twilio.

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
- **Fix:** Either remove `RealIP` if the server is exposed directly, or configure a trusted proxy list.

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
- **Fix:** Validate path is under a whitelist directory and reject files over a max size.

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
- **Fix:** Append milliseconds or a random suffix.

### H12. Timezone mismatch in message statistics
- **File:** `internal/db/messages.go`
- **Lines:** 365-375
- **Issue:** `GetStats` uses SQLite `date('now')` and `datetime('now', '-7 days')`, which operate in UTC. However, `created_at` is stored as local time from `time.Now()`. For any non-UTC server, the "today", "this_week", and "this_month" counts will be wrong by the timezone offset.
- **Fix:** Pass `time.Now().UTC()` for all DB writes, or use `datetime('now', 'localtime')` in the query.

### H13. Missing ON DELETE cascade breaks cleanup
- **File:** `internal/db/migrations/001_initial_schema.up.sql`
- **Issue:** `messages.did_id`, `cdrs.did_id`, `cdrs.device_id`, and `voicemails.cdr_id` / `voicemails.user_id` reference parent tables without `ON DELETE CASCADE` or `ON DELETE SET NULL`. With foreign keys enabled, deleting a DID, device, CDR, or user that has child records causes a hard constraint error instead of a clean cascade.
- **Fix:** Add appropriate `ON DELETE` actions or document the required deletion order.

### H14. Missing rows.Err() check
- **File:** `internal/db/sessions.go`
- **Lines:** 150-191
- **Issue:** `ListByUserID` returns `sessions, nil` after `rows.Next()` finishes. If `rows.Next()` stopped because of a database error, that error is silently swallowed.
- **Fix:** Return `sessions, rows.Err()`.

### H15. Partial-update booleans silently disable features
- **Files:** `internal/api/messages.go:430`, `internal/api/routes.go:184`, `internal/api/provisioning.go:407`
- **Issue:** Update handlers use plain `bool` (not `*bool`) in request structs. Omitting the `enabled` field in JSON sets it to `false`, silently disabling auto-replies, routes, or profiles.
- **Fix:** Use `*bool` for optional boolean fields in update requests, or fetch existing record and only mutate provided fields.

### H16. JWT stored in localStorage — XSS vector
- **File:** `frontend/src/api/client.ts`
- **Line:** 16
- **Issue:** The JWT is read from `localStorage`. If any XSS vector were introduced (or a malicious browser extension is present), the token is trivially exfiltrated.
- **Fix:** Move session management to `httpOnly`, `Secure`, `SameSite=Strict` cookies.

### H17. Audio element memory leak + race condition
- **File:** `frontend/src/views/VoicemailsView.vue`
- **Lines:** 122-151
- **Issue:** `playVoicemail()` overwrites `audioRef` without removing the previous element's `onended` listener. If a user clicks a different voicemail while one is playing, the old audio element remains in memory and its `onended` handler will later corrupt state (setting `playingId` and `audioRef` to null at the wrong time). Additionally, there is no `onUnmounted` cleanup: if the component unmounts while audio is playing, the Audio DOM object continues to exist and its closure references prevent garbage collection.
- **Fix:** Track the active `Audio` instance in a local variable, remove listeners on pause/stop, and clean up in `onUnmounted`.

### H18. Frontend settings form clears stored passwords
- **File:** `frontend/src/views/SettingsView.vue`
- **Lines:** 81-100, 166-180
- **Issue:** `loadSystemSettings` always populates password fields (`twilio_auth_token`, `smtp_password`, `gotify_token`) with empty strings. `saveSystemSettings` then sends those empty strings to `PUT /system/config`. If the backend does not explicitly ignore empty passwords, this will overwrite and clear credentials.
- **Fix:** Strip empty password fields from the payload before sending.

---

## Medium (13 issues)

### M1. Nil context passed to DB layer
- **File:** `internal/api/webhooks.go`
- **Lines:** 319, 394
- **Issue:** `h.deps.DB.Config.Get(nil, "voicemail_greeting")` and `h.deps.DB.Devices.GetByID(nil, deviceID)` pass `nil` context. This can cause panics or undefined behavior if the DB implementation relies on context deadlines or cancellation.
- **Fix:** Pass `r.Context()` instead of `nil`.

### M2. Widespread ignored errors
- **Files:** Many across `internal/api/`
- **Representative examples:**
  - `webhooks.go` 94-103: `CDRs.Update` error ignored — call records inconsistent
  - `webhooks.go` 119: `strconv.ParseInt(didIDStr)` error ignored — `didID` becomes 0, voicemail attached to wrong DID
  - `webhooks.go` 134: `Voicemails.Create` error ignored — voicemail may be lost
  - `webhooks.go` 229: `Messages.Create` error ignored — inbound SMS may be lost
  - `auth.go` 229: `Users.Count` error ignored — pagination total wrong
  - `system.go` 64: `fmt.Sscanf(portStr)` error ignored — invalid port silently accepted
- **Fix:** Check errors and at minimum log them; for writes, return 500 to the client.

### M3. Lockout duration uses oldest attempt, not newest
- **File:** `internal/api/auth.go`
- **Lines:** 410-418
- **Issue:** `lockoutEnd := recent[0].Add(config.LoginLockoutDuration)` — `recent[0]` is the oldest failed attempt. Lockout timing is wrong.
- **Fix:** Use `recent[len(recent)-1]` (the most recent attempt).

### M4. 409 Conflict returned for arbitrary DB errors
- **Files:** `internal/api/dids.go:89`, `internal/api/auth.go:286`
- **Issue:** `Create` handlers return HTTP 409 for any DB error, not just unique-constraint violations. Disk-full, locked DB, and other server errors are misreported as client conflicts.
- **Fix:** Inspect the error type / message and return 500 for non-constraint errors.

### M5. Cookie Secure flag breaks behind reverse proxy
- **File:** `internal/api/auth.go`
- **Line:** 116
- **Issue:** `Secure: r.TLS != nil` is false when running behind a TLS-terminating reverse proxy. Session cookies are sent without the Secure flag, making them vulnerable to interception on insecure networks.
- **Fix:** Check `X-Forwarded-Proto` header or add a config flag for secure cookies.

### M6. Session cache never cleaned in production
- **File:** `internal/api/middleware.go`
- **Lines:** 125-264
- **Issue:** `cleanupExpiredSessions` exists but nothing calls it on a timer. The in-memory cache grows forever.
- **Fix:** Start a background ticker on server initialization.

### M7. Backup directories world-readable
- **File:** `internal/db/db.go`
- **Lines:** 87, 279
- **Issue:** `os.MkdirAll(backupsDir, 0755)` creates backups with world-read permissions. Backups contain password hashes, auth tokens, call records, and message content.
- **Fix:** Use `0750` or stricter.

### M8. Blocklist check is O(n) and loads entire table
- **File:** `internal/db/blocklist.go`
- **Lines:** 98-131
- **Issue:** `IsBlocked` calls `r.List(ctx)` (full table scan) then iterates all entries in Go for every single number check. This becomes a DoS vector as the blocklist grows.
- **Fix:** Push pattern matching into the database with `LIKE` for prefix/exact and a regex extension for regex, or cache the blocklist.

### M9. Schema drift after rolling back migration 005
- **File:** `internal/db/migrations/005_add_linphone_device_type.down.sql`
- **Issue:** The down migration recreates `devices` but only restores `idx_devices_username` and `idx_devices_user_id`. It omits `idx_devices_mac` and `idx_devices_provisioning_status` added in `003.up`.
- **Fix:** Recreate all indexes that exist in the up path.

### M10. SIP header injection in MWI NOTIFY
- **File:** `pkg/sip/server.go`
- **Lines:** 424-430
- **Issue:** `SendMWINotify` builds `Contact`, `From`, `To`, and `CSeq` headers with `fmt.Sprintf` using subscription fields without sanitizing SIP special characters (newlines, `<`, `>`). If any of these fields are attacker-controlled, arbitrary SIP headers can be injected.
- **Fix:** Validate or escape SIP special characters in subscription fields.

### M11. Nil pointer risks in MWI handlers
- **File:** `pkg/sip/handlers_mwi.go`
- **Lines:** 129-134, 159-163, 195-202
- **Issue:** `resp.To()`, `req.Via()`, `req.From()` accessed without nil checks. Malformed SIP requests cause nil-pointer panics.
- **Fix:** Add nil guards before dereferencing.

### M12. SIPTrunkHandler nil pointer + no escaping
- **File:** `internal/api/webhooks.go`
- **Lines:** 515-536
- **Issue:** `url.Parse("sip:" + to)` error is ignored; `toURI.User` may be nil. Also `from` is injected into TwiML without escaping at line 533.
- **Fix:** Check the error, validate `toURI.User != nil`, and escape values in TwiML.

### M13. Frontend unhandled promise rejections
- **Files:** `frontend/src/views/ProvisioningView.vue:223-225` (clipboard), `frontend/src/views/VoicemailsView.vue:141` (audio play)
- **Issue:** `navigator.clipboard.writeText(text)` and `audioRef.value.play()` return Promises that are never caught. If the clipboard API is blocked or audio playback fails, unhandled rejections are thrown.
- **Fix:** Add `.catch(...)` or wrap in `try/await`.

---

## Low (5 issues)

### L1. No max-length / format validation on many inputs
- **Files:** `internal/api/auth.go`, `internal/api/devices.go`, `internal/api/dids.go`, `internal/api/system.go`, `internal/api/tls.go`
- **Examples:**
  - `CreateUserRequest.Email` — no email format check
  - `CreateDeviceRequest.Name/Username/Password` — no max length
  - `CreateDIDRequest.Number` — no phone-number format check
  - `TLSConfigRequest.CertFile/KeyFile` — no path traversal check
- **Fix:** Add appropriate validators.

### L2. Static file server masks API 404s
- **File:** `internal/api/routes.go`
- **Line:** 281
- **Issue:** `r.Handle("/*", http.FileServer(http.Dir("./frontend/dist")))` catch-all means unknown API routes return the SPA HTML instead of a proper 404, confusing API clients.
- **Fix:** Serve static files under a specific prefix (e.g., `/app/*`) or check the route first.

### L3. Migration sort is lexicographic, not numeric
- **File:** `internal/db/db.go`
- **Lines:** 133-144
- **Issue:** The code relies on `os.ReadDir` lexicographic sorting. Unpadded filenames like `10_` sort before `2_`, potentially applying migrations out of order.
- **Fix:** Parse version numbers and sort numerically.

### L4. Malformed migration filenames break tracking
- **File:** `internal/db/db.go`
- **Line:** 150
- **Issue:** `fmt.Sscanf(filename, "%d_", &version)` return value is ignored. A file named `patch.up.sql` parses as version `0`, causing incorrect migration state.
- **Fix:** Check the return value and skip/abort on bad filenames.

### L5. Frontend incorrect multipart Content-Type
- **File:** `frontend/src/api/calls.ts`
- **Lines:** 129-142, 146-159
- **Issue:** Manually setting `headers: { 'Content-Type': 'multipart/form-data' }` on an axios request that sends `FormData` strips the required `boundary` parameter, which can cause the server to fail parsing the upload.
- **Fix:** Remove the manual header and let axios set the full multipart header automatically.

---

## Summary

| Severity | Count |
|----------|-------|
| Critical | 7 |
| High | 18 |
| Medium | 13 |
| Low | 5 |
| **Total** | **43** |

---

## Priority Fix Order

1. Escape all user values in TwiML (`webhooks.go`)
2. Add signature validation to every webhook (`webhooks.go`)
3. Recover panics in all fire-and-forget goroutines
4. Fix `RestoreBackup` race condition (`internal/db/db.go`)
5. Fix migration 003 down to preserve provisioning data
6. Add auth checks to SIP BYE/CANCEL/REFER
7. Fix `string(rune(req.SMTPPort))` -> `strconv.Itoa`
8. Add `offset >= 0` / `limit > 0` validation to all list endpoints
9. Fix frontend open redirect and password-clearing bugs
10. Implement incoming call routing in `handleInvite`
