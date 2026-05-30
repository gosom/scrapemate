# Browser Session Pool Design

**Date:** 2026-04-12

**Goal:** Reduce RAM growth, CPU overhead, and Chromium process count for browser-heavy workloads without breaking the public API or changing the default session-preservation behavior.

## Problem

The current Playwright fetcher eagerly creates `Concurrency` browser instances at startup and keeps one long-lived browser context per browser slot. That preserves cookies and storage, but it also means:

- browser count scales directly with configured concurrency
- browsers and contexts can live indefinitely and accumulate memory over long runs
- pages are usually recreated per job, so the runtime pays repeated page setup cost without fully benefiting from reuse
- when the pool is empty, the fetcher can create additional ad hoc browsers, which allows process growth beyond the intended steady state

These trade-offs are expensive for `github.com/gosom/google-maps-scraper`, where the browser path dominates cost and long-running stability matters more than raw startup simplicity.

## Constraints

- Keep the public API unchanged.
- Keep `scrapemateapp.WithJS(...)` behavior source-compatible.
- Preserve session state by default.
- Prefer automatic improvements over new knobs.
- Favor stability over aggressive browser sharing.

## Architecture

Replace the current eager browser pool with a lazy session-slot pool.

Each session slot owns:

- one Playwright browser process
- one persistent browser context
- one primary reusable page
- lifecycle metadata used to decide when to clean or recycle the slot

The browser context remains the session boundary. Cookies, storage, and other browser state continue to live there. A job still runs against a single `BrowserPage`, and the fetcher still exposes the same `HTTPFetcher` contract to the rest of the system.

The key behavior change is internal:

- browsers are created lazily when a slot is first used
- a healthy primary page is reused by default
- extra pages created by jobs are cleaned up before the slot returns to the pool
- unhealthy pages, contexts, or browsers are recycled in a deterministic order
- the fetcher stops creating unbounded extra browsers when all slots are busy

## Components

### Session Slot

`sessionSlot` replaces the current thin `browser` holder. It tracks:

- `browser`
- `context`
- `page`
- page reuse count
- browser/context age or usage count
- last cleanup result
- recent recovery attempts

Its responsibility is to provide one stable execution environment for one job at a time.

### Slot Pool

The fetcher maintains a bounded pool of session slots sized to the intended concurrency. Slots are acquired and released, but they are initialized lazily. If all slots are busy, fetchers wait for an available slot instead of creating more long-lived browsers.

### Cleaner

After every job, cleanup restores the slot to a known-good steady state:

- keep one primary page
- close extra pages
- verify the primary page is still open and usable

Cleanup preserves session state because it does not reset the context unless the context is already unhealthy.

### Recycler

Recycling is ordered by the cheapest recovery first:

1. recreate the page
2. recreate the context and page
3. recreate the browser, context, and page

This keeps the stable fast path cheap and makes degraded slots recover deterministically.

## Data Flow

For each browser-backed job:

1. Acquire a slot from the pool.
2. Lazily initialize the slot if it has not been used before.
3. Ensure the slot has a healthy browser, context, and primary page.
4. Run `job.BrowserActions(ctx, page)` using the primary page.
5. Clean the slot:
   leave one primary page and close extras.
6. Recycle only if health checks or recovery heuristics require it.
7. Return the slot to the pool.

This changes the current behavior where browsers are eagerly created up front and the fetcher may exceed the pool size under pressure.

## Stability And Error Handling

The design is intentionally conservative:

- no slot is shared across concurrent jobs
- no session state is shared across slots
- page failures only reset the page when possible
- context resets happen only when page recovery is insufficient
- full browser replacement is reserved for disconnected or repeatedly unhealthy slots

One compatibility trade-off remains explicit: if a slot must recreate its browser context for health reasons, that slot loses its session state. This is acceptable because the same outcome already exists today when the underlying browser dies unexpectedly.

## Testing

The implementation should add deterministic unit tests around the slot lifecycle by introducing an internal factory seam instead of requiring real Playwright browsers in every test.

Required coverage:

- lazy slot creation does not allocate browsers until first use
- pool acquisition blocks at capacity instead of growing browser count
- a healthy page is reused across jobs
- extra pages are closed before returning the slot
- broken pages trigger page recreation
- disconnected browsers trigger full slot recreation
- cleanup or recovery failures do not return an unusable slot to the pool

Integration coverage should remain small and focused:

- `scrapemateapp` still wires JS fetchers from existing config
- current config options continue to work unchanged

## Rollout

This work should be delivered in one focused change set to the JS fetcher internals and app wiring tests. Multi-context-per-browser optimization is intentionally out of scope for this iteration. If this stability-first change is insufficient, that broader packing strategy can be considered later with measurements in hand.
