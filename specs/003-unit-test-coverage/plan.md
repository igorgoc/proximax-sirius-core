# Implementation Plan: Complete Go Backend Test Coverage

**Branch**: `003-unit-test-coverage` | **Date**: 2026-09-17 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/003-unit-test-coverage/spec.md`

## Summary

Implement standalone, hermetic unit tests for `pkg/migrator`, `pkg/network`, and `pkg/storage` to complete Go test coverage across the backend without relying on live external networks.

## Technical Context

**Language/Version**: Go 1.22+

**Testing Tools**: `testing` standard library, `os.MkdirTemp` for file system isolation, `httptest` for network isolation.

**Target Packages**:
- `backend/pkg/migrator`
- `backend/pkg/network`
- `backend/pkg/storage`

## Constitution Check

- [x] **Key Privacy**: Tests assert that `storage.GetStatus()` always removes private keys.
- [x] **Storage & Fast-Sync**: Tests verify binary index file layouts without modifying live chain files.
- [x] **Hermetic Execution**: Tests use temporary directories (`t.TempDir()`) and local mock servers (`httptest.Server`) to guarantee zero host pollution.

## Project Structure

```text
backend/pkg/
├── migrator/
│   ├── migrator.go
│   └── migrator_test.go      # [NEW: Unit test suite for flat-file migration]
├── network/
│   ├── upnp.go
│   └── upnp_test.go          # [NEW: Unit test suite for port reachability]
└── storage/
    ├── storage.go
    └── storage_test.go       # [NEW: Unit test suite for properties & metrics]
```
