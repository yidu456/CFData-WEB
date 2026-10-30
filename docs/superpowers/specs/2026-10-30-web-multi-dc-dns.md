# Web Multi-Datacenter Testing and Huawei DNS Publishing

## Approved scope

Deliver the functional Web version first. CLI extensions and Windows scheduling are deferred until the user reviews the Web version. Preserve the established visual design and existing single-DC/GitHub/edgetunnel flows.

## Testing and selection

The top datacenter control is a dropdown with checkboxes for any subset of the dynamically discovered datacenters, synchronized with the datacenter table. It is independent of the legacy auto-speed switch; before discovery it explains that scanning supplies the choices. Keep any legacy single-center auto target separately and clearly labelled, shown only for its existing mode. FRA and SJC are examples only, never a hardcoded allowlist. The Web task accepts a configurable maximum detailed latency in milliseconds, minimum download speed in MB/s, and final total result limit (default 10). Measure all latency-qualified selected candidates before ranking; never stop at the first 10 qualified addresses. Combine, deduplicate by IP, exclude failed/unmeasured or below-threshold results, sort speed descending then latency ascending with deterministic ties, and retain the top N across all selected DCs. No per-DC quota; fewer than N is valid. Missing DCs and zero qualified results are reported without fallback to other DCs. Results retain datacenter metadata. Publish only completed backend-owned results; cancellation or failed/incomplete tasks cannot publish.

## Web Huawei DNS configuration

Provide a real configuration panel: AK/SK inputs, supported service endpoint/region configuration, read-only connection test, public-zone discovery, recordset discovery/selection, record identity/type display, optional private backend persistence, update preview, manual confirmed update, and opt-in update after a successful batch. Empty credential inputs preserve saved credentials; never return SK or full AK in configuration/status responses. Never store keys in localStorage, exported configuration, logs, tracked sources, or ordinary result files. New privileged APIs require same-origin protection plus local-only access unless the existing Web authentication is enabled. General frontend-only checks do not establish permission.

Use the official Huawei SDK and service-specific authentication. Update explicitly selected existing A/AAAA recordsets only, verify identity and family, preserve TTL/line/other properties, compare IP sets and skip unchanged updates. No create/delete/recreate operation. A is pure IPv4; AAAA is pure IPv6; DNS values never include ports. Empty/no matching family results preserve old DNS. Bound requests and return sanitized errors. Connection tests only read cloud state; updating requires the user action/opt-in.

## Publishing and delivery

Reuse GitHub's existing configuration/upload UI; support manual and opt-in completion publishing of selected results. GitHub and DNS attempts/statuses are independent. No live scan, real DNS mutation, real result upload, or schedule installation during automated QA. Use mocked network/browser flows; produce a Windows preview build separate from the user's existing EXE, start only a loopback preview, and expose it for user testing. Tests, vet, formatting checks, and Linux/Windows builds precede each step's commit/push to yidu456/CFData-WEB dev. Never modify main, beta, .github/, or reference-only root Go files.
