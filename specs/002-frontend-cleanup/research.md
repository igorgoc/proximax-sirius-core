# Research: Frontend Cleanup and Chunking Strategy

## Decision 1: Deletion of `SnapshotModal.tsx`
- **Rationale**: `SnapshotModal.tsx` was the initial floating modal design. It was completely superseded by `SnapshotSubTab.tsx` embedded in `MaintenanceTab.tsx`. No other component references it.
- **Alternatives Considered**: Keeping as backup (Rejected: causes dead code accumulation).

## Decision 2: Rollup Output `manualChunks`
- **Rationale**: `lucide-react` is a large icon library (~1,500 icon variants) contributing ~350 kB of minified JS. Separating `lucide-react` and `react-vendor` (`react`, `react-dom`) into dedicated cached chunks reduces the primary application bundle to <150 kB, allowing faster incremental browser caching and eliminating Vite warnings.
