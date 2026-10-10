#!/usr/bin/env python3
"""
Hermetic Mock Sirius REST Server for Super CI Integration Tests.
Provides deterministic API responses for /chain/height, /account/:id,
and /transactions/confirmed without contacting public mainnet.
"""

import sys
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 3000

# In-memory mock state
state = {
    "height": 14065880,
    "accounts": {
        # Alice (Legitimate 200,000 XPX Delegator)
        "ECF38DD76309CF010FB27866AE651E7579BB13AE213AACBCAAAB510BC7E9482B": {
            "address": "B827612836327E6ACC021D748D395F5AE28C5CA66B58CDA25A",
            "accountType": 1,
            "linkedAccountKey": "6D072E1E3FA64AE46B7E62D702B79E959B0968142BBC082DE3E5C255F7B44E99",
            "balance": 200000000000,  # 200,000 XPX
        },
        "XATWCKBWGJ7GVTACDV2I2OK7LLRIYXFGNNMM3IS2": {
            "address": "B827612836327E6ACC021D748D395F5AE28C5CA66B58CDA25A",
            "accountType": 1,
            "linkedAccountKey": "6D072E1E3FA64AE46B7E62D702B79E959B0968142BBC082DE3E5C255F7B44E99",
            "balance": 200000000000,
        },
        # Bob (Underfunded 5,000 XPX Account)
        "XBOBUNDERFUNDED0000000000000000000000000": {
            "address": "B8B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0B0",
            "accountType": 1,
            "linkedAccountKey": "0000000000000000000000000000000000000000000000000000000000000000",
            "balance": 5000000000,  # 5,000 XPX
        },
        # Node Harvest Key (Remote Harvester)
        "1D339BA5E197D7AB2E4BFA9312B5C115040740F9F00C5E3BD7EA6F911B5827F2": {
            "address": "B838A9C7ABD2AEEABFC286FC509B7CA28D7FC56EBF5E05F0FE",
            "accountType": 2,
            "linkedAccountKey": "F8C33546AB26A2ED132F8D12FBE10168B775A879BD0699F3EAFF2A1EBE40F43E",
            "balance": 0,
        },
        "XCVTDLG4RQZPT62KHXZUC4EUNRKZ36VGQJJXPYLX": {
            "address": "B838A9C7ABD2AEEABFC286FC509B7CA28D7FC56EBF5E05F0FE",
            "accountType": 2,
            "linkedAccountKey": "F8C33546AB26A2ED132F8D12FBE10168B775A879BD0699F3EAFF2A1EBE40F43E",
            "balance": 0,
        },
        # Node Main Account
        "XA4KTR5L2KXOVP6CQ36FBG34UKGX7RLOX5PAL4H6": {
            "address": "B838A9C7ABD2AEEABFC286FC509B7CA28D7FC56EBF5E05F0FE",
            "accountType": 1,
            "linkedAccountKey": "1D339BA5E197D7AB2E4BFA9312B5C115040740F9F00C5E3BD7EA6F911B5827F2",
            "balance": 500000000000,
        },
    },
    "confirmed_transactions": [],
}

class MockSiriusHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        # Silence routine request logging in CI output
        pass

    def _json(self, data, code=200):
        body = json.dumps(data).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path

        if path == "/chain/height":
            state["height"] += 1
            h = state["height"]
            self._json({"height": [h & 0xFFFFFFFF, (h >> 32) & 0xFFFFFFFF]})
            return

        if path.startswith("/account/"):
            target = path.split("/account/")[1].strip("/").upper()
            acc = state["accounts"].get(target)
            if not acc:
                # Default generic active account
                acc = {
                    "address": target,
                    "accountType": 1,
                    "linkedAccountKey": "6D072E1E3FA64AE46B7E62D702B79E959B0968142BBC082DE3E5C255F7B44E99",
                    "balance": 200000000000,
                }
            bal = acc["balance"]
            low = bal & 0xFFFFFFFF
            high = (bal >> 32) & 0xFFFFFFFF
            self._json({
                "meta": {},
                "account": {
                    "address": acc.get("address", target),
                    "accountType": acc.get("accountType", 1),
                    "linkedAccountKey": acc.get("linkedAccountKey", ""),
                    "mosaics": [
                        {"id": [2679028825, 1076571991], "amount": [low, high]}
                    ]
                }
            })
            return

        if path == "/transactions/confirmed":
            self._json({"data": state["confirmed_transactions"]})
            return

        if path == "/health":
            self._json({"status": "healthy", "height": state["height"]})
            return

        self._json({"error": "not found"}, 404)

    def do_POST(self):
        parsed = urlparse(self.path)
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length) if length > 0 else b""
        payload = json.loads(body.decode("utf-8")) if body else {}

        if parsed.path == "/mock/inject_tx":
            state["confirmed_transactions"].insert(0, payload)
            self._json({"status": "injected", "count": len(state["confirmed_transactions"])})
            return

        if parsed.path == "/mock/clear_txs":
            state["confirmed_transactions"] = []
            self._json({"status": "cleared"})
            return

        if parsed.path == "/mock/set_account":
            acc_id = str(payload.get("account", "")).upper()
            if acc_id:
                state["accounts"][acc_id] = {
                    "address": payload.get("address", acc_id),
                    "accountType": payload.get("accountType", 1),
                    "linkedAccountKey": payload.get("linkedAccountKey", ""),
                    "balance": payload.get("balance", 200000000000),
                }
            self._json({"status": "account_updated", "account": acc_id})
            return

        if parsed.path == "/mock/set_balance":
            acc_id = str(payload.get("account", "")).upper()
            new_bal = payload.get("balance", 0)
            if acc_id in state["accounts"]:
                state["accounts"][acc_id]["balance"] = new_bal
            self._json({"status": "updated", "account": acc_id, "balance": new_bal})
            return

        if parsed.path == "/transactions":
            self._json({"message": "packet announced", "status": "Success"})
            return

        self._json({"error": "not found"}, 404)

if __name__ == "__main__":
    server = HTTPServer(("127.0.0.1", PORT), MockSiriusHandler)
    print(f"[Mock Sirius API] Server listening on http://127.0.0.1:{PORT}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        server.server_close()
        print("[Mock Sirius API] Server stopped.", flush=True)
