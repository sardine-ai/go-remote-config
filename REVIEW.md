# REVIEW.md

Rules for code review in this repository. Reviewers (human or automated) must check every change against these.

## Rules

### GetData values must never be mutated in place

The `client` package caches `GetConfig` conversions (`client/cache.go`). Cache validity is decided by `sameSource`, which compares map/slice identity (pointer and length), not content. Nothing enforces this at compile or run time, so correctness depends on the contract documented on `source.Repository.GetData` (`source/repository.go`):

- A `Repository` must publish new data by replacing the whole map/slice returned from `GetData` on `Refresh()`.
- A `Repository` must never edit a previously returned value in place (nested map assignment, in-capacity `append`, slice element writes).
- Unmarshal outside the lock, then swap the stored data under the lock.

An in-place mutation leaves `sameSource` true, so `GetConfig` silently returns the stale cached conversion forever.

When reviewing, flag any of the following as a bug:

- A new or changed `source.Repository` implementation that mutates data returned by `GetData` instead of replacing it.
- A change to `client/cache.go` (`sameSource`, `buildEntry`, `rebuildCache`) that weakens or bypasses the identity check, or that adds a code path mutating cached or source values.
- Removal or weakening of the immutability comment on `GetData` or in `client/cache.go`.
- New tests or fakes whose `GetData` returns a map/slice that the test later edits in place and then expects `GetConfig` to observe.
