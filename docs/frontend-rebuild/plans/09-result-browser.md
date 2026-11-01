# Shared Result Browsing Implementation Plan

> TDD/review/verify/build/local commit then output configuration. No push.

**Goal:** Browse every stored official scan/detail and NSB row with independent filters, three-level stable sort and pagination; do not change task/measurement/output scope.

**Architecture:** A shared pure browser model selects original row indexes. Shared native controls own per-table view preference/page only; official/NSB state stays authoritative and whole. Three separate browser instances retain their choices across mode/navigation.

**Spec:** R01-R08/R10, existing strictly-below latency/loss thresholds, default0/100 unlimited. Official custom latency order min/max/avg, speed descending numeric then untested then failure, duplicate priorities dedupe. IPv4/6 classification based endpoint literal. Current source UI preserved; no styling overhaul.

## Steps

- [x] Node RED tests `browseResultRows(rows,options,type,headers)` strict threshold filtering, unknown fails active filter, qualification from configured threshold, IPtype, priority dedupe/stable tie, failure/untested speed order, official nano unit handling, NSB string metrics, whole pages of105rows/last page/clamp and no source mutation.
- [x] Implement `web/features/results/browser.js` pure metric/selection. Returns `{indexes,total,page,pages}` with pageSize100; never returns a mutated or filtered replacement for task inputs.
- [x] Implement `web/features/results/controls.js` native per-table IP/status/latency/loss/sort1..3 controls and prev/next/status. Controls integrate into official scan/detail and NSB views by selecting indexes before rendering; drop recent100-only restriction, preserve all current columns. Three browser states distinct. Memoize source revision between control actions; whole dataset work occurs on changed source/preferences, not per action repeatedly.
- [x] Browser mocks105rows pagination/full retained counts, filtered/sorted results, mode preservation, batch/relink dispatch still105 rows while view1row. 320/375/768/1366 responsive/native focus and existing regressions.
- [x] Go/node/vet/browser/review no blockers, explicit embed tests, Windows/LinuxCGO0test build. Local commit `Add shared result filters sorting and pagination` then output/clipboard/download slice.

## Limits

IP details/copy, independent output filters, fields/range/formats are next slice. Current feature count baseline remains112; no history/backend pagination/queue introduced. Browser pagination retains all data in memory; genuinely huge result sets may still benefit from CLI.

## Verification Record (2026-11-02)

- 51 Node tests, Go uncached tests/vet and Windows/LinuxCGO0 test builds pass. New browser module/controls initially absent (RED); zero numeric speed ordering regression failed before fix then passed.
- Three per-table native views select original indexes, strict latency/loss/status/IP family filters, stable deduped three-priority sort and100-row pages. Source data/whole-task scope not changed.
- Browser mock verifies105rows pageable, strict view filter to1row still dispatches105 measurement targets, mode persistence, and threshold change1MB/s qualified visible→hidden at2MB/s without another event. Existing all suites pass, including four viewport widths/screenshots.
- Full filtered/sorted index cache excludes page number and shared empty headers stable. Independent instrumentation reads105metrics only once across next/previous pages, new source revision recomputes210 total. NSB speed threshold input rerenders immediately. Independent re-review no blockers.
- No copy/export/publishing or IP-detail action yet; next output slice handles these. Live public scans/downloads/APK/CLI schedule untested. Artifacts ignored and no push.
