# Quickstart: Pure Browser Verification

## Verification Steps

1. Build frontend:
   ```bash
   cd frontend
   npm run build
   ```

2. Run Go test suite with race detection:
   ```bash
   cd backend
   go test -race -count=1 ./pkg/...
   ```

3. Start supervisor and open in browser:
   ```bash
   ./scripts/macos/start.sh
   # Open browser: http://localhost:8080
   ```
