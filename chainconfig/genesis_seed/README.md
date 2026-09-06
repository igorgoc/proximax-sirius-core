# ProximaX Sirius Mainnet Genesis Seed Package

This directory contains the authentic ProximaX Sirius Mainnet Genesis (Block 1) seed files required to initialize a new peer node from Block 0/1.

## Files Included:
* **`00000/00001.dat`**: The raw Block 1 Nemesis payload (13,381 bytes).
* **`00000/hashes.dat`**: The 32-byte Block 1 cryptographic generation hash and block hash.
* **`index.dat`**: The 8-byte 64-bit integer index pointing to Height 1.

## How It Works with `feature/reducedDataSize`:
When a node starts with this seed package:
1. Block 1 is loaded from `00000/00001.dat`.
2. All ongoing blocks downloaded from peers (Height 2 to Height 13.8M+) are streamed directly into:
   * `00000/blocks.dat` (Concatenated block element payload)
   * `00000/statements.dat` (Concatenated statement receipts)
   * `00000/blocks.idx` (Direct 16-byte fixed index lookup table)
3. Zero individual loose block files are created on the host filesystem.
