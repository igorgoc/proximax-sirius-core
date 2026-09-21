# Research & Test Design Decisions

## Decision 1: Migrator Hermetic Test Isolation
- Use `t.TempDir()` to construct chunk directories (`00000`, `00001`), populate dummy loose `00001.dat` and `00001.stmt` files.
- Verify that `blocks.dat`, `statements.dat`, and `blocks.idx` are created, the 16-byte index matches little-endian offsets, and loose files are removed.

## Decision 2: Storage Manager Test Isolation
- Use `t.TempDir()` for `resourcesPath` and `dataPathGetter`.
- Verify property round-tripping (`SaveStorageConfig` -> `LoadStorageConfig`), `GetStorageMetrics` directory walk calculations, `CleanSandboxes`, and key redaction in `GetStatus`.

## Decision 3: Network Manager Test Isolation
- Test `PortCheckResult` struct creation, thread-safe `GetLastResult()`, and local port listening probes with standard ephemeral ports.
