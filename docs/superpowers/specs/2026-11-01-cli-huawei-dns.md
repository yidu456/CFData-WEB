# CLI Multi-Center Testing And Huawei DNS

## Approved Workflow

Scan once, test the union of explicitly selected data centers, measure speed,
merge qualified results, sort by measured speed descending and latency ascending,
and select an inclusive rank range (normally 1 through 10) for one existing Huawei
public-zone A/AAAA record. There are no per-center quotas. Rank selection is over
the measured qualified pool, not every address on the Internet.

## Compatibility And Scope

- Keep the original Web UI and algorithms, retained results, CLI exports, GitHub,
  edgetunnel, nonstandard mode, and installed scheduling untouched.
- Extend `offdc` to a comma-separated list. Normalize uppercase, deduplicate,
  reject malformed codes before scanning, and report missing selected centers.
  Blank keeps the existing automatic lowest-latency center behavior.
- Complete all selected centers' latency testing before one combined speed pass.
  `offspeedlimit` remains the global qualified speed-result limit (default 5).
  Users who want ten DNS IPs must measure at least ten qualified results; examples
  use a larger measured pool. Never silently increase a user's test limit.
- Reuse the existing exact-measurement selection and Huawei SDK/store code.
  DNS publishes only measured, finite, threshold-qualified, matching-family bare
  IPs. Deduplicate and globally sort before taking the DNS rank range.
- Initial DNS integration is for official CLI runs, including a single center;
  enabling it in nonstandard mode or file-only upload/compaction is rejected.
- No new dependencies, whole-UI redesign, batch per-center caps, DNS creation,
  TTL/line/weight changes, or automatic cloud writes during development.

## Configuration And Authority

- CLI DNS options live in `cliHuaweiConfig`; the CLI JSON template uses a nested
  `huaweiDNS` object with `enabled`, `zone`, `name`, `type`, `line`, `mode`,
  `startRow`, and `endRow`. Defaults: disabled, A, default_view, overwrite, 1, 10.
- Flags: `huawei`, `hwzone`, `hwname`, `hwtype`, `hwline`, `hwmode`,
  `hwstartrow`, `hwendrow`, and invocation-only `hwconfirm`. Flags override file
  values; explicit false/empty values must not retain a prior menu invocation.
- DNS rank selection is separate from export `outstartrow`/`outendrow` and export
  qualification. Existing defaults (including export end row 20) do not change.
- Reuse executable-local `huawei-dns.private`. No AK/SK fields or secret flags in
  plaintext CLI JSON. Add a separate CLI credential configuration menu; explicitly
  saving credentials uses the existing authenticated application encryption.
- Environment overrides `CFDATA_HUAWEI_AK`, `CFDATA_HUAWEI_SK`,
  `CFDATA_HUAWEI_REGION`, `CFDATA_HUAWEI_PROJECT_ID` are memory-only and are never
  printed. Both keys are required when replacing credentials. An invalid private
  file fails visibly, not overwritten or silently replaced with blank credentials.
- `hwconfirm` is never saved in JSON. Default: show the current cloud record and
  planned final IPs, then ask for confirmation. EOF/back/no means no update.
  `-hwconfirm=true` explicitly authorizes this invocation's previewed update for
  unattended runs. A cancelled scan/task must never publish.

## Target And Mutation Safety

Discover one public zone by exact normalized name. Select exactly one existing
record by name, type, and line. Missing or ambiguous matches fail before mutation.
Overwrite replaces only IP values; append preserves cloud IPs before new selected
IPs and deduplicates. Empty selection never clears DNS. Display old/new IPs before
confirmation. Update reuses configuration revision, record identity/fingerprint,
context cancellation, and records-only SDK mutation checks. Cloud changes after
preview require another invocation/preview; do not retry a stale mutation.

DNS answer order is controlled by resolvers and cannot be guaranteed. Submission
order follows the chosen ranking, but reordering an unchanged IP set is a no-op.

## Verification And Delivery

Use Go 1.26.2, `CGO_ENABLED=0`, focused failing tests before implementation, full
Go tests/vet and Windows/Linux whole-package builds at each code step. Mock only
network boundaries; test real orchestration, selection, SDK discovery/preview/
update, file config resolution, and menu input with temporary files. Run existing
frontend regressions at final delivery. Work in the existing linked `dev` worktree;
commit the plan and every completed implementation step. Do not push new commits
without further authorization. Never operate on main or protected `.github/`.
