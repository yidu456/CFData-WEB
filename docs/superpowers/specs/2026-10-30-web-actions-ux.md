# Web Result Actions and Datacenter UX

## Approved Scope

Keep scanning before datacenter selection and the existing full-candidate global top-N measurement/publication rules. Add search inside the existing discovered-datacenter multi-selector; no locations-based preselection, automatic all-center selection, or new scan semantics. Preserve the established design and backend publication security.

## Result Operations

Opening Results Operations shows four entries only: export file, upload GitHub, upload edgetunnel, update Huawei DNS. Each enters its dedicated configuration view with a back action. Reuse existing form nodes, IDs, account storage, previews, and upload functions; never duplicate credentials or change upload contracts. Shared output formatting controls may appear within the selected output view, not as a combined all-service form. Huawei uses its existing panel and is reachable from the chooser even with zero results: configuration/read-only discovery remains available, updates still require a valid completed backend batch. Preserve legacy quick-file/NSB/single-center behavior and batch provenance guards. Navigation never triggers an upload/update; focus, Escape/back, desktop/mobile access are verified.

## Datacenter Selection

Add a labelled search input to the discovered DC dropdown. Filter by code/city/country case-insensitively; typing alone changes no selection. Clicking a matching option toggles membership, retains selected centers outside the query, and synchronizes table controls. Clear query shows all discovered options. Explicit empty/no-matches states. Retain existing required selection and task-running disable behavior. Make table checkbox 24px with at least 44x44 clickable label area, clear checked/focus/hover/disabled styles, shallow row highlight, no row-click interference with single-center test buttons. Use actual accessible checkbox semantics and no dependencies.

## Encrypted Huawei Configuration

Reuse Windows current-user DPAPI and the independent OS-user configuration file cfdata-web/huawei-dns.private. Encrypt the complete existing basic config (AK, SK, region, projectId), not only its display mask. Provide an explicit Save Encrypted Configuration action in Huawei's view; preserve memory-only Apply and blank-credential preservation. Expose only save capability, safe location/status, never plaintext AK/SK. Do not move encrypted material into ordinary browser/CLI exports. Saving never performs discovery, update, or uploads. Save/reload/overwrite/tamper/failure tests must preserve last good config/file and log no secrets. Existing Windows ciphertext remains compatible. Non-Windows has no encrypted persistence implementation: fail closed on new persistent writes rather than calling plaintext permissions encryption; memory use remains available. Do not add a password/key manager or silently convert legacy files.

## Delivery

Reuse isolated dev worktree; commit and push every reviewed task to yidu456/CFData-WEB dev. No main/beta/protected files/production EXE changes. Tests use network/browser mocks only. Build Windows/Linux after each step; final preview uses independent loopback executable on port 13336 after verifying process identity. No live scan/cloud mutation/upload or scheduler installation. CLI/scheduling remain deferred.
