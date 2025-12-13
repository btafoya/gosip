# GoSIP Specification Expert Panel Review

**Review Date**: 2025-12-13
**Specification**: REQUIREMENTS.md (545 lines)
**Review Mode**: Critique
**Overall Quality Score**: 7.2/10

---

## Expert Panel

| Expert | Domain | Focus Area |
|--------|--------|------------|
| **Karl Wiegers** | Requirements Engineering | Clarity, testability, acceptance criteria |
| **Michael Nygard** | Production Systems | Reliability, failure modes, operations |
| **Martin Fowler** | Architecture & Design | API design, schema quality, patterns |
| **Gojko Adzic** | Specification by Example | Executable examples, scenarios |
| **Sam Newman** | Service Integration | Twilio patterns, boundaries |

---

## Executive Summary

The GoSIP REQUIREMENTS.md provides a **solid foundation** with clear scope, well-structured feature tables, and a comprehensive database schema. However, the specification lacks **operational hardening** - specifically around performance requirements, failure handling, and testable acceptance criteria.

### Strengths
- Clear project scope (2-5 phones, 1-3 DIDs)
- Well-structured feature tables
- Complete 12-table database schema
- Comprehensive API endpoint coverage
- Clear technology decisions

### Critical Gaps
1. No performance/SLA requirements
2. No failure mode specifications
3. Vague acceptance criteria
4. Missing operational/observability requirements
5. No executable test scenarios

---

## Detailed Expert Analysis

### Karl Wiegers - Requirements Quality

**Quality Assessment**: 6.8/10

#### Critical Issues

| ID | Severity | Issue | Impact |
|----|----------|-------|--------|
| RQ-001 | HIGH | No performance requirements | Cannot validate system meets expectations |
| RQ-002 | HIGH | Vague acceptance criteria | Requirements not testable |
| RQ-003 | MEDIUM | Missing error handling specs | Unknown system behavior on failure |
| RQ-004 | MEDIUM | Incomplete security specs | Potential vulnerabilities |

#### RQ-001: Performance Requirements Missing

**Current State**: No SLAs specified anywhere in the document.

**Recommendation**: Add "Performance Requirements" section:

```markdown
## Performance Requirements

| Metric | Requirement | Measurement |
|--------|-------------|-------------|
| SIP Registration Time | < 500ms | Time from REGISTER to 200 OK |
| Call Setup Latency | < 2 seconds | Time from INVITE to media flow |
| API Response (GET) | < 200ms | 95th percentile |
| API Response (POST) | < 500ms | 95th percentile |
| Concurrent Calls | 5 minimum | Simultaneous active calls |
| System Startup | < 30 seconds | Time to accept first registration |
```

#### RQ-002: Vague Acceptance Criteria

**Current State**:
- "NAT Handling: Symmetric RTP, rport support" - Success undefined
- "Spam filtering: Twilio spam score threshold" - No threshold specified
- "Recording: On-demand" - Trigger mechanism unclear

**Recommendation**: Convert to testable statements:

```markdown
### Call Blocking - Testable Requirements

| Requirement | Acceptance Criteria |
|-------------|---------------------|
| Spam Blocking | System SHALL reject calls with Twilio spam_score > 0.7 |
| Anonymous Rejection | System SHALL reject calls with empty/blocked caller ID when enabled |
| Blacklist | System SHALL reject calls matching blacklist within 100ms |

### Recording - Testable Requirements

| Trigger | Behavior |
|---------|----------|
| Per-call button | Recording starts within 2 seconds of activation |
| Device setting | All calls to/from device are automatically recorded |
| Route rule | Calls matching route condition are recorded |
```

#### RQ-003: Missing Error Handling

**Recommendation**: Add error handling matrix:

```markdown
## Error Handling Requirements

| Error Condition | Detection | Response | User Notification |
|-----------------|-----------|----------|-------------------|
| Twilio API 5xx | HTTP status | Retry 3x with backoff | Alert after 3 failures |
| Twilio timeout | 30s timeout | Queue operation | Silent |
| Invalid SIP credentials | 401 response | Reject registration | Device error display |
| Database write failure | SQLite error | Retry, then read-only mode | Admin alert |
```

#### RQ-004: Security Specifications

**Current State**:
- "Rate limiting on auth endpoints" - No limits specified
- "Session timeout" - No duration
- "HTTPS recommended" - Should be required

**Recommendation**:

```markdown
## Security Requirements (Specific)

| Control | Specification |
|---------|---------------|
| Auth Rate Limit | 5 failed attempts/minute, 15-minute lockout |
| Session Timeout | 24 hours, refresh on activity |
| Password Policy | Minimum 8 characters, 1 uppercase, 1 number |
| HTTPS | REQUIRED for production deployments |
| SIP TLS | Optional, configurable per device |
| Webhook Validation | Twilio signature validation REQUIRED |
```

---

### Michael Nygard - Production Systems

**Quality Assessment**: 5.5/10

#### Critical Issues

| ID | Severity | Issue | Impact |
|----|----------|-------|--------|
| OP-001 | CRITICAL | No failure mode analysis | Unknown system behavior |
| OP-002 | HIGH | Missing observability | Cannot monitor/debug |
| OP-003 | MEDIUM | Backup/recovery incomplete | Data loss risk |
| OP-004 | MEDIUM | No capacity planning | Resource issues |

#### OP-001: Failure Mode Analysis

**Recommendation**: Add comprehensive failure matrix:

```markdown
## Failure Modes & Recovery

| Component | Failure Mode | Detection | Response | Recovery Time |
|-----------|--------------|-----------|----------|---------------|
| Twilio API | Unreachable | HTTP timeout | Queue requests, alert | Auto-retry 60s |
| Twilio API | Rate limited | 429 response | Backoff, queue | Auto-retry with exponential backoff |
| SQLite | Corruption | Integrity check | Read-only mode | Manual restore from backup |
| SQLite | Disk full | Write error | Reject new recordings | Admin intervention |
| SIP Port | Blocked | No registrations | Alert admin | Network config required |
| Device | Offline | Registration expiry | Route to voicemail | Auto-reregister on reconnect |
| Recording | Storage full | Write error | Stop recording, continue call | Admin cleanup |

### Circuit Breaker Configuration

| Service | Failure Threshold | Recovery Time | Fallback |
|---------|-------------------|---------------|----------|
| Twilio Voice | 5 failures/minute | 60 seconds | Queue calls |
| Twilio SMS | 5 failures/minute | 60 seconds | Queue messages |
| Twilio Recording | 3 failures/minute | 120 seconds | Skip recording |
| Email (SMTP) | 3 failures | 300 seconds | Queue notifications |
| Push (Gotify) | 3 failures | 60 seconds | Silent fail |
```

#### OP-002: Observability Requirements

**Recommendation**:

```markdown
## Observability Requirements

### Logging

| Category | Log Level | Retention | Format |
|----------|-----------|-----------|--------|
| SIP Transactions | INFO | 7 days | JSON with call_id |
| API Requests | INFO | 30 days | JSON with request_id |
| Authentication | WARN | 90 days | JSON with user_id |
| Errors | ERROR | 90 days | JSON with stack trace |
| Twilio Webhooks | DEBUG | 7 days | JSON with webhook_sid |

### Metrics

| Metric | Type | Alert Threshold |
|--------|------|-----------------|
| Active Registrations | Gauge | < 1 for > 5 minutes |
| Calls Per Hour | Counter | N/A |
| API Latency P95 | Histogram | > 1 second |
| Error Rate | Gauge | > 10% |
| Twilio API Latency | Histogram | > 5 seconds |
| Disk Usage | Gauge | > 80% |

### Health Check Endpoint

GET /api/system/health

Response:
{
  "status": "healthy|degraded|unhealthy",
  "components": {
    "database": { "status": "healthy", "latency_ms": 5 },
    "sip_listener": { "status": "healthy", "registrations": 3 },
    "twilio": { "status": "healthy", "last_check": "2025-01-01T12:00:00Z" }
  },
  "version": "1.0.0",
  "uptime_seconds": 86400
}
```

#### OP-003: Backup & Recovery

**Recommendation**:

```markdown
## Backup & Recovery Requirements

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| RTO (Recovery Time) | 1 hour | Acceptable for home office |
| RPO (Recovery Point) | 24 hours | Daily backup sufficient |
| Backup Frequency | Daily at 3 AM | Low activity period |
| Backup Retention | 7 days | 1 week recovery window |

### Backup Contents

| Data | Included | Location |
|------|----------|----------|
| SQLite database | Yes | /app/data/gosip.db |
| Call recordings | Yes | /app/data/recordings/ |
| Voicemail audio | Yes | /app/data/voicemails/ |
| Configuration | Yes | Database config table |

### Recovery Procedure

1. Stop GoSIP container
2. Restore gosip.db from backup
3. Restore media directories
4. Verify database integrity: `sqlite3 gosip.db "PRAGMA integrity_check"`
5. Start GoSIP container
6. Verify registrations resume
```

#### OP-004: Capacity Planning

**Recommendation**:

```markdown
## Resource Requirements

### Minimum System Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 1 core | 2 cores |
| Memory | 256 MB | 512 MB |
| Disk | 1 GB | 10 GB |
| Network | 1 Mbps | 10 Mbps |

### Growth Estimates

| Data Type | Growth Rate | 1 Year Projection |
|-----------|-------------|-------------------|
| CDR Records | ~50/day | ~18,000 records (~18 MB) |
| Recordings | ~10 min/day | ~3.6 GB (at 1MB/min) |
| Voicemails | ~2/day | ~730 files (~730 MB) |
| SMS Messages | ~20/day | ~7,300 records (~7 MB) |
```

---

### Martin Fowler - Architecture & API Design

**Quality Assessment**: 7.5/10

#### Issues

| ID | Severity | Issue | Impact |
|----|----------|-------|--------|
| AR-001 | MEDIUM | API pagination missing | Large result sets problematic |
| AR-002 | MEDIUM | Schema indexes not specified | Query performance |
| AR-003 | LOW | External API auth undefined | Security gap |
| AR-004 | LOW | Error response format undefined | Inconsistent client handling |

#### AR-001: API Pagination

**Recommendation**: Add to API specifications:

```markdown
## API Conventions

### Pagination

All list endpoints support pagination:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| page | integer | 1 | Page number (1-indexed) |
| per_page | integer | 20 | Items per page (max 100) |
| sort | string | created_at | Sort field |
| order | string | desc | Sort order (asc/desc) |

Response headers:
- X-Total-Count: Total items
- X-Total-Pages: Total pages
- Link: Pagination links (RFC 5988)

### Filtering

CDR endpoints support filtering:

| Parameter | Type | Description |
|-----------|------|-------------|
| from_date | ISO8601 | Start date filter |
| to_date | ISO8601 | End date filter |
| direction | string | inbound/outbound |
| disposition | string | answered/missed/blocked/voicemail |
```

#### AR-002: Database Indexes

**Recommendation**: Add to schema:

```sql
-- Performance indexes
CREATE INDEX idx_cdrs_started ON cdrs(started_at DESC);
CREATE INDEX idx_cdrs_disposition ON cdrs(disposition);
CREATE INDEX idx_cdrs_did ON cdrs(did_id);
CREATE INDEX idx_messages_created ON messages(created_at DESC);
CREATE INDEX idx_messages_did ON messages(did_id);
CREATE INDEX idx_voicemails_user ON voicemails(user_id);
CREATE INDEX idx_voicemails_read ON voicemails(is_read);
CREATE INDEX idx_registrations_device ON registrations(device_id);
CREATE INDEX idx_registrations_expires ON registrations(expires_at);
CREATE INDEX idx_routes_did_priority ON routes(did_id, priority);
```

#### AR-003: External API Authentication

**Recommendation**:

```markdown
## External API Authentication

### API Key Management

| Field | Description |
|-------|-------------|
| key_id | Unique identifier (UUID) |
| key_hash | Bcrypt hash of API key |
| name | Human-readable name |
| scopes | Allowed operations (read, write, calls, messages) |
| expires_at | Optional expiration |
| last_used | Last access timestamp |

### Authentication Header

Authorization: Bearer <api_key>

### Rate Limits

| Scope | Limit | Window |
|-------|-------|--------|
| Read | 100 requests | 1 minute |
| Write | 20 requests | 1 minute |
| Calls | 10 requests | 1 minute |
```

#### AR-004: Standard Response Format

**Recommendation**:

```markdown
## API Response Format

### Success Response

{
  "data": { ... },
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 150
  }
}

### Error Response

{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid phone number format",
    "details": [
      { "field": "to_number", "message": "Must be E.164 format" }
    ]
  }
}

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| NOT_FOUND | 404 | Resource does not exist |
| VALIDATION_ERROR | 400 | Request validation failed |
| UNAUTHORIZED | 401 | Authentication required |
| FORBIDDEN | 403 | Insufficient permissions |
| RATE_LIMITED | 429 | Too many requests |
| SERVER_ERROR | 500 | Internal server error |
```

---

### Gojko Adzic - Specification by Example

**Quality Assessment**: 6.0/10

#### Issues

| ID | Severity | Issue | Impact |
|----|----------|-------|--------|
| EX-001 | HIGH | No executable examples | Cannot validate requirements |
| EX-002 | MEDIUM | Edge cases undefined | Ambiguous behavior |
| EX-003 | MEDIUM | Voicemail scenarios missing | Incomplete feature spec |

#### EX-001: Executable Examples

**Recommendation**: Add scenario section:

```markdown
## Behavior Specifications

### Call Routing Scenarios

**Scenario: Business Hours Routing**
```gherkin
Given DID "555-1234" has business hours 9:00-17:00 Monday-Friday
And device "office-phone" is registered
When inbound call arrives at 14:30 on Tuesday
Then call SHALL ring "office-phone" for 30 seconds
And on no answer, call SHALL route to voicemail
```

**Scenario: After Hours Forwarding**
```gherkin
Given DID "555-1234" has after-hours forwarding to "+15555678"
When inbound call arrives at 20:00 on Tuesday
Then call SHALL forward to "+15555678"
And CDR SHALL record original caller ID
```

**Scenario: DND Active**
```gherkin
Given DND is enabled for DID "555-1234"
When any inbound call arrives
Then call SHALL route directly to voicemail
And no device SHALL ring
```

### Call Blocking Scenarios

**Scenario: Spam Score Blocking**
```gherkin
Given blocking rule "reject if spam_score > 0.7"
When call arrives with Twilio spam_score 0.85
Then call SHALL be rejected
And caller SHALL hear "disconnected" tone
And CDR SHALL record disposition "blocked_spam"
```

**Scenario: Blacklist Blocking**
```gherkin
Given "+15551234567" is on blacklist
When call arrives from "+15551234567"
Then call SHALL be rejected within 100ms
And CDR SHALL record disposition "blocked_blacklist"
```

### SMS Auto-Reply Scenarios

**Scenario: After Hours Auto-Reply**
```gherkin
Given auto-reply rule "after_hours" with message "We're closed. Back at 9 AM."
And current time is 21:00
When SMS arrives on DID "555-1234"
Then system SHALL send auto-reply within 5 seconds
And original message SHALL be stored normally
```

**Scenario: Rate-Limited Auto-Reply**
```gherkin
Given sender received auto-reply 30 minutes ago
When same sender sends another SMS
Then system SHALL NOT send another auto-reply
And original message SHALL be stored normally
```
```

#### EX-002: Edge Cases

**Recommendation**:

```markdown
## Edge Case Specifications

### Phone Number Handling

| Scenario | Behavior |
|----------|----------|
| International number | Accept E.164 format only (+1..., +44...) |
| Local number | Reject - require country code |
| Short code | Accept for SMS only |
| Emergency (911) | Pass through to Twilio without modification |

### Registration Conflicts

| Scenario | Behavior |
|----------|----------|
| Same device, different IP | Update registration, invalidate old |
| Same credentials, different device | Reject second registration |
| Expired registration + active call | Maintain call until BYE |

### Concurrent Operations

| Scenario | Behavior |
|----------|----------|
| SMS during active call | Process independently |
| Recording request during recording | Ignore duplicate request |
| Multiple devices ring | First answer wins, others get BYE |
```

#### EX-003: Voicemail Specifications

**Recommendation**:

```markdown
## Voicemail Specifications

### Trigger Conditions

| Condition | Voicemail Trigger |
|-----------|-------------------|
| No answer | After 30 seconds (configurable) |
| All devices offline | Immediate |
| User rejection | Immediate |
| DND enabled | Immediate |

### Recording Limits

| Parameter | Value | Configurable |
|-----------|-------|--------------|
| Max length | 180 seconds | Yes |
| Min length | 3 seconds | No (shorter discarded) |
| Silence timeout | 10 seconds | No |

### Greeting Options

| Type | Description |
|------|-------------|
| System default | "Please leave a message after the tone" |
| Per-DID custom | User-uploaded audio file |
| Per-user custom | Future enhancement |

### Notification Timing

| Notification | Timing |
|--------------|--------|
| Push (Gotify) | Within 5 seconds of recording end |
| Email | Within 60 seconds (after transcription) |
```

---

### Sam Newman - Service Integration

**Quality Assessment**: 7.0/10

#### Issues

| ID | Severity | Issue | Impact |
|----|----------|-------|--------|
| SI-001 | HIGH | Twilio webhook handling undefined | Potential message loss |
| SI-002 | MEDIUM | Health check coverage incomplete | Unknown dependency status |
| SI-003 | MEDIUM | Notification boundaries unclear | Inconsistent behavior |

#### SI-001: Twilio Webhook Handling

**Recommendation**:

```markdown
## Twilio Webhook Specifications

### Signature Validation

All webhooks MUST validate Twilio signature:
- Header: X-Twilio-Signature
- Algorithm: HMAC-SHA1 of URL + sorted POST params
- On failure: Return 403, log attempt

### Idempotency

| Field | Usage |
|-------|-------|
| CallSid | Unique per voice webhook |
| MessageSid | Unique per SMS webhook |
| RecordingSid | Unique per recording webhook |

Behavior:
- Store processed SIDs for 24 hours
- On duplicate: Return 200, skip processing
- Log duplicate attempts

### Response Requirements

| Webhook Type | Max Response Time | Response Format |
|--------------|-------------------|-----------------|
| Voice | 10 seconds | TwiML |
| SMS | 10 seconds | TwiML or empty |
| Status | 5 seconds | Empty 200 |
| Recording | 5 seconds | Empty 200 |

### Retry Handling

Twilio retries on:
- Timeout (10+ seconds)
- 5xx response
- Connection failure

GoSIP behavior:
- Accept retries gracefully (idempotent)
- Log retry attempts
- Alert on >3 retries for same webhook
```

#### SI-002: Dependency Health Checks

**Recommendation**:

```markdown
## External Dependency Health

### Startup Checks

| Check | Required | Timeout | On Failure |
|-------|----------|---------|------------|
| SQLite accessible | Yes | 5s | Abort startup |
| SIP port bindable | Yes | 1s | Abort startup |
| Twilio credentials valid | Yes | 10s | Start degraded |
| Webhook URLs reachable | No | 5s | Log warning |

### Runtime Checks

| Dependency | Interval | Method | Unhealthy After |
|------------|----------|--------|-----------------|
| Twilio API | 5 min | GET /2010-04-01/Accounts/{sid} | 3 failures |
| SMTP server | 15 min | EHLO | 2 failures |
| Gotify | 5 min | GET /health | 3 failures |

### Degraded Mode Behavior

| Dependency Down | Behavior |
|-----------------|----------|
| Twilio | Queue outbound, accept inbound via SIP |
| SMTP | Queue emails, retry hourly |
| Gotify | Skip push notifications |
| SQLite | Read-only mode, reject new data |
```

#### SI-003: Notification Matrix

**Recommendation**:

```markdown
## Notification Specifications

### Event to Notification Mapping

| Event | Email | Push | SMS Forward | Admin Alert |
|-------|-------|------|-------------|-------------|
| Missed call | Config | Yes | No | No |
| Voicemail | Yes | Yes | No | No |
| New SMS | Config | Yes | Config | No |
| Device offline | No | No | No | Yes |
| Service degraded | No | No | No | Yes |
| Storage >80% | No | No | No | Yes |

### User Preferences Schema

```sql
notification_prefs (
  user_id INTEGER REFERENCES users(id),
  event_type TEXT,
  email_enabled BOOLEAN DEFAULT TRUE,
  push_enabled BOOLEAN DEFAULT TRUE,
  quiet_hours_start TIME,
  quiet_hours_end TIME
)
```

### Quiet Hours Behavior

- During quiet hours: Queue notifications
- At quiet hours end: Send summary (not individual)
- Emergency: Ignore quiet hours (admin alerts)
```

---

## Quality Scorecard

| Category | Score | Expert |
|----------|-------|--------|
| Requirements Clarity | 6.8/10 | Wiegers |
| Production Readiness | 5.5/10 | Nygard |
| Architecture Quality | 7.5/10 | Fowler |
| Testability | 6.0/10 | Adzic |
| Integration Design | 7.0/10 | Newman |
| **Overall** | **7.2/10** | Panel |

---

## Improvement Roadmap

### Immediate (Before Development)

| Priority | Issue | Effort | Impact |
|----------|-------|--------|--------|
| P0 | Add performance requirements | 2 hours | High |
| P0 | Add failure mode matrix | 4 hours | High |
| P0 | Specify security parameters | 2 hours | High |
| P1 | Add database indexes | 1 hour | Medium |
| P1 | Define API response format | 1 hour | Medium |

### Short-Term (During Phase 1-2)

| Priority | Issue | Effort | Impact |
|----------|-------|--------|--------|
| P1 | Add executable test scenarios | 4 hours | High |
| P1 | Add observability requirements | 3 hours | High |
| P2 | Define pagination conventions | 1 hour | Medium |
| P2 | Add Twilio webhook specs | 2 hours | Medium |

### Long-Term (During Implementation)

| Priority | Issue | Effort | Impact |
|----------|-------|--------|--------|
| P2 | Add capacity planning | 2 hours | Medium |
| P2 | Define notification matrix | 2 hours | Medium |
| P3 | Add edge case documentation | 3 hours | Low |
| P3 | External API authentication | 2 hours | Low |

---

## Expert Panel Consensus

The panel unanimously agrees:

1. **Foundation is solid** - Good scope, clear architecture, comprehensive schema
2. **Operational gaps are critical** - Must address before production deployment
3. **Testability needs work** - Add concrete scenarios for all behaviors
4. **Twilio integration well-conceived** - But needs failure handling details

**Recommendation**: Address P0 issues before starting Phase 1 implementation. This specification is 70% complete - the remaining 30% is operational hardening that will prevent production issues.

---

## Appendix: Quick Reference

### Must-Add Sections

1. Performance Requirements (SLAs)
2. Failure Modes & Recovery
3. Observability Requirements
4. Security Specifications (with values)
5. API Conventions (pagination, errors)
6. Behavior Specifications (Given/When/Then)

### Schema Additions

```sql
-- Add to registrations
last_seen DATETIME DEFAULT CURRENT_TIMESTAMP

-- Add to cdrs
notes TEXT

-- Add indexes (see AR-002)
```

### New Tables Suggested

```sql
-- API keys for external access
api_keys (
  id INTEGER PRIMARY KEY,
  key_hash TEXT NOT NULL,
  name TEXT,
  scopes TEXT,
  created_at DATETIME,
  expires_at DATETIME,
  last_used DATETIME
)

-- Notification preferences
notification_prefs (
  user_id INTEGER REFERENCES users(id),
  event_type TEXT,
  email_enabled BOOLEAN,
  push_enabled BOOLEAN,
  quiet_hours_start TIME,
  quiet_hours_end TIME
)

-- Webhook deduplication
processed_webhooks (
  sid TEXT PRIMARY KEY,
  processed_at DATETIME,
  webhook_type TEXT
)
```
