# Executable-Local Storage and Huawei DNS UI

## Approved Direction

Continue in the existing isolated `dev` checkout at `422fde5`. The user approved
executable-local storage, including saved Web preferences and upload accounts,
and requested a clearer Huawei page with its duplicate entry removed. The latest
annotation preserves browser downloads: Web exports retain their current
Blob/download behavior and browser-selected destination.

This changes persistence flows, not just paths. Reuse existing atomic file writes,
Windows DPAPI, account formats and authenticated WebSocket operations. No database,
new dependencies or global `os.Chdir`.

## Storage and Security

- Resolve program-owned default files from the executable directory:
  `cfdata-config.json`, `huawei-dns.private`, IP lists, `locations.json`, ASN
  database, existing program-generated results and debug logs.
- Add executable-local `web-state.private` for saved form, theme/autoload and
  GitHub/edgetunnel account data. Encrypt the complete state with current-user
  DPAPI and preserve existing account unlock/upload behavior.
- Default reads, writes, cache cleanup and temporary files must agree on the
  directory. Preserve existing explicit input/output-path semantics; no new CLI
  options, workflows or scheduler installation.
- Display effective storage location and truthful save status. Unwritable storage
  must fail explicitly, preserving good files/state; never fall back to AppData,
  plaintext or browser persistence. Non-Windows encrypted writes fail closed;
  memory-only use remains available. DPAPI files cannot migrate across users or
  machines simply by copying the executable folder.

## Web State and Compatibility

New saves go to the backend, not localStorage. Reads/writes require the existing
strict same-origin local-access or active-login checks. Validate and bound state;
serialize atomic updates so an account or theme change preserves unrelated
sections. Never log credentials or include them in normal exports, generic status
responses or errors.

Support explicit migration from old browser data and the previous AppData Huawei
file. Never overwrite existing executable-local data with a stale browser copy.
Only a clearly labelled user action after successful saving may clean up the old
source; failures preserve both sources. Do not silently convert old non-Windows
plaintext into purported encrypted storage.

Loading accounts/preferences grants no publishing authority. Reload, reconnect
and migration must not restore automatic cloud-write opt-ins or convert historical
results into a current completed batch. Preserve session/provenance guards,
preview/confirmation and independent operation statuses.

Web exports still download in the browser. Do not introduce a server export-save
endpoint, automatic result history or duplicate export files. Existing transient
tasks and browser-selected input File objects remain transient.

## Huawei Page

Keep only `Result Operations -> Update Huawei DNS`. Remove the settings-menu
`Huawei DNS configuration and preview` shortcut and its orphaned heading. Keep
configuration reachable with empty results.

Reorganize existing fields, without duplicating credentials, into three sections:

1. Connection: AK/SK, region and Project ID; separate memory Apply, encrypted Save
   and read-only connection test, each with short explanatory text.
2. DNS target: discover public zones and existing A/AAAA records; show selected
   name, type, line and TTL clearly.
3. Preview/update: current batch, old/new IP comparison and explicit confirmation;
   explain optional automatic publishing separately.

Add verified official links beside AK/SK, Project ID, region and zone/record fields.
Use a separate tab with `noopener noreferrer`; no credentials in link URLs. Keys
must be created manually in Huawei's console, not retrieved through the DNS API.
Reuse read-only APIs for zones/records.

Preserve established typography/colors and focus/back/Escape behavior. Separate
configuration, discovery and update statuses; technical endpoint/encryption
explanations go in expandable help. Verify desktop and narrow mobile layouts.

## Validation and Delivery

After user review, write a sequential implementation plan:

1. Shared default paths and compatible Huawei-file migration.
2. Encrypted backend Web state, frontend saves/loads and explicit migration.
3. Huawei page grouping, verified help links and duplicate-entry removal.
4. Integrated tests, documentation and builds.

Test different launch directories, cache cleanup, failed writes, DPAPI reload and
tamper, migration failures, authorization, preserved publishing guards, unchanged
browser downloads and keyboard/mobile UI. Review, test, build Windows/Linux, commit
and push each implementation step to `yidu456/CFData-WEB` `dev`.

No `main`, protected files, production EXE replacement, scheduler installation,
real scans, uploads or DNS changes. Use mocks. Prior preview launch attempts were
blocked by the execution tool policy; do not bypass it or claim a running preview
without an allowed launch and successful HTTP/UI checks.
