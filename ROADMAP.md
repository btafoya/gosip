# ROADMAP.md — Endpoints, Provisioning, and Core Call Control

This roadmap focuses on improving GoSIP’s “desk-phone first” user experience: faster onboarding, reliable call control, and tighter integration with hardware capabilities (keys, lamps, and screen branding).

---

## Product goals

1. **Time-to-first-call under 10 minutes**
   - A device can be onboarded via wizard (assisted) or by DHCP-driven auto-provisioning.
2. **Expected PBX call controls**
   - Hold/resume with Music-on-Hold (MOH)
   - Blind transfer and attended transfer
3. **Hardware-native user experience**
   - Use phone hardware buttons for hold/transfer where possible
   - Use the voicemail Message Waiting Indicator (MWI) lamp
   - Nice-to-have: custom logo on supported phones

---

## Guiding principles

- **GoSIP owns the lifecycle**: endpoint identity, credentials, provisioning, and call-control state transitions.
- **Standards first, vendor quirks isolated**: implement via SIP standards; keep vendor specifics in profiles/templates.
- **Observable by default**: provisioning fetches, registrations, hold/transfer actions, and MWI updates produce structured events.
- **Secure by default**: provisioning endpoints handle secrets; favor tokened URLs with expiry and revocation.

---

## Roadmap themes

### Theme A — Endpoint provisioning (assisted + automatic)
**Assisted provisioning**
- Web wizard generates:
  - SIP settings (server/transport/user/auth/pass)
  - vendor/model field mappings (“where to paste each value”)
  - a concise copy block
- Wizard supports credential rotation and re-onboarding steps (factory reset guidance per vendor)

**Automatic provisioning**
- GoSIP hosts a provisioning endpoint that serves vendor-specific configurations.
- Default access pattern: **tokened provisioning URL** (short-lived, revocable).
- UI outputs DHCP option guidance (Option 66 baseline) and a provisioning URL.

**Operational visibility**
- Per-device status: last config fetch attempt, last registration seen, errors, recent events.

---

### Theme B — Core call control (hold, transfer, MOH)
**Hold / Resume**
- SIP re-INVITE/UPDATE hold semantics, interoperable with common desk phones.
- MOH is delivered to held parties (looped default track).
- Resume restores two-way audio deterministically.

**Transfer**
- Blind transfer: transfer active call to destination.
- Attended transfer: consult call, then complete/cancel.
- Failure handling: clear user messaging and best-effort rollback when a transfer fails.

---

### Theme C — Hardware feature integration (MWI, keys, logo)
This theme focuses on surfacing PBX functionality through capabilities users expect on physical phones.

#### C1 — Voicemail Message Waiting Indicator (MWI lamp)
Goal: voicemail notifications light up on phones automatically.

- Implement SIP **MWI/Message Waiting** using standards-based signaling:
  - Notify the registered endpoint when voicemail state changes (new messages / cleared)
- Ensure behavior supports:
  - turning lamp on for new messages
  - turning lamp off when mailbox is cleared or messages are marked read
- Tie MWI to a mailbox model:
  - per-user mailbox state
  - events on message arrival/deletion/read
- UI:
  - mailbox status surface (counts, last updated)
  - troubleshooting indicators (endpoint supports MWI? last MWI notify result?)

Acceptance criteria
- Leave a voicemail → lamp turns on for the target extension.
- Clear voicemail → lamp turns off.
- Works across at least one desk phone and one softphone that supports MWI.

#### C2 — Use phone hold/transfer buttons (feature interop)
Goal: pressing Hold/Transfer on the phone works as expected without requiring the web UI.

- Support phone-initiated call control through SIP-standard mechanisms:
  - Hold button triggers appropriate SIP signaling; GoSIP updates session state and MOH behavior.
  - Transfer button triggers SIP transfer flows; GoSIP orchestrates blind/attended patterns depending on what the phone initiates.
- Implementation planning notes:
  - Many phones implement transfer via SIP `REFER` (and sometimes vendor-specific UX flows). GoSIP must translate these into the internal call-control state machine.
  - Ensure GoSIP can handle both:
    - GoSIP-initiated transfers (from web UI)
    - Phone-initiated transfers (from handset buttons)

Acceptance criteria
- Hold button on phone places call on hold and MOH plays to the held party.
- Transfer button on phone can complete at least blind transfer between extensions.
- GoSIP UI reflects the correct call state during phone-initiated actions.

#### C3 — Nice-to-have: custom screen logo (vendor templates)
Goal: allow branding on supported desk phones during provisioning.

- Add optional per-tenant or global branding:
  - logo URL or uploaded asset
  - vendor-specific config keys to set idle screen logo / background (varies per vendor)
- Restrict initial scope:
  - implement for the first supported vendor/model profile only
  - document limitations clearly
- UX:
  - “Branding” section in provisioning wizard with preview
  - fallback to “disabled” when the phone does not support it

Acceptance criteria (nice-to-have)
- Provisioning config includes the correct vendor settings to display a logo.
- A supported phone shows the custom logo after provisioning/reboot.

---

## Delivery plan (incremental milestones)

### Milestone 0 — Foundations
- Establish a single call session state machine with explicit hold/transfer states.
- Establish a unified event/log stream for:
  - provisioning lifecycle
  - registrations
  - hold/transfer/MOH actions
  - voicemail state changes (later)
- UI scaffolding for Provisioning and In-Call Control panels.

### Milestone 1 — Assisted provisioning (first vendor/model)
- Wizard produces correct SIP settings and per-field mapping.
- Device registers successfully.
- Credential rotation supported.

### Milestone 2 — Automatic provisioning (tokened URLs)
- Provisioning endpoint serves rendered configs securely.
- UI provides provisioning URL and DHCP option guidance.
- Fetch status visible per device.

### Milestone 3 — Hold/Resume + MOH
- Hold/resume works reliably with MOH to held parties.
- UI hold/resume controls shipped.

### Milestone 4 — Blind transfer
- Web UI initiated blind transfer works.
- Failure states are observable and user-friendly.

### Milestone 5 — Attended transfer (consult)
- Consult call + complete/cancel shipped.
- MOH applied correctly during consult.

### Milestone 6 — MWI lamp (voicemail notification)
- Voicemail state changes trigger MWI updates to endpoints.
- UI surfaces mailbox and MWI troubleshooting.

### Milestone 7 — Phone button interop (hold/transfer from device)
- Phone-initiated hold and transfer flows supported.
- UI stays in sync with device-initiated actions.

### Milestone 8 — Nice-to-have: branding/logo (first vendor/model)
- Optional logo configured in wizard and deployed via provisioning template.

---

## Testing and compatibility plan (high level)

- Start with a single desk phone family and one softphone and expand.
- For each newly supported vendor/model profile:
  - assisted provisioning validation
  - automatic provisioning validation
  - hold/resume + MOH validation
  - transfer validation (web-initiated and phone-initiated when applicable)
  - MWI validation

---

## Documentation deliverables
- Endpoint provisioning guide (assisted + automatic)
- DHCP option guidance (generic, not router-specific)
- Call control guide (hold, transfer, MOH)
- Voicemail + MWI guide (how it works, troubleshooting)
- Vendor/model specific pages (template behaviors, button quirks, logo support)
