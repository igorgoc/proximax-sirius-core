# Quickstart: Frontend Cleanup Validation

## Commands
1. Build frontend:
   ```bash
   cd frontend
   npm run build
   ```
   *Expected Outcome*: Clean build with separate chunks for `react-vendor` and `lucide-icons`, with no chunk exceeding 500 kB.

2. Run backend test suite:
   ```bash
   cd backend
   go test ./...
   ```
   *Expected Outcome*: All tests pass.
