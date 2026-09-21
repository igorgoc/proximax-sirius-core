# Research & Dead Code Audit Findings

## Target 1: `backend/cmd/fast-extract/`
- **Decision**: Remove.
- **Rationale**: Standalone CLI test binary built before `backend/pkg/snapshot` was created. Production streaming restore is performed directly inside the Go supervisor (`pkg/snapshot/manager.go`).
- **Impact Assessment**: Zero dependencies inside `backend/` or `frontend/`.

## Target 2: `backend/cmd/stream-chunk-restore/`
- **Decision**: Remove.
- **Rationale**: Experimental CLI tool for chunk-by-chunk download tests. Unused in production runtime or packaging scripts.
- **Impact Assessment**: Zero runtime references.

## Target 3: `scripts/common/audit_chunks.py`
- **Decision**: Remove.
- **Rationale**: Standalone Python validator for `blocks.idx`/`blocks.dat` format. Sirius Catapult stores block data in `00000/*.dat` and `index.dat` directly with RocksDB column families.
- **Impact Assessment**: Orphaned script, never invoked by packagers or node operators.

## Target 4: `scripts/linux/fast-snapshot.sh`
- **Decision**: Remove.
- **Rationale**: Contains Docker commands (`docker compose stop/start sirius-core`). In Sirius Native, snapshot downloads are managed natively on the host via the Cockpit UI without Docker.
- **Impact Assessment**: No callers in native packaging.

## Target 5 & 6: `scripts/linux/test_config_persistence.sh` & `test_transition_persistence.sh`
- **Decision**: Remove.
- **Rationale**: Prototype bash test scripts written early in supervisor development. Superseded by comprehensive Go unit test suites in `pkg/config/config_test.go` and `pkg/supervisor/supervisor_test.go`.
- **Impact Assessment**: Replaced by standard `go test ./...` workflow.
