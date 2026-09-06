#!/usr/bin/env python3
import os
import sys
import struct
import time
import json
from pathlib import Path

def audit_storage(data_dir):
    print("=" * 60)
    print("  ProximaX Sirius Blockchain Storage Full Integrity Audit")
    print(f"  Target Data Directory: {data_dir}")
    print("=" * 60)

    start_time = time.time()
    
    # 1. Read index.dat
    index_path = os.path.join(data_dir, "index.dat")
    if not os.path.exists(index_path):
        print(f"Error: index.dat not found in {data_dir}")
        return False
    
    with open(index_path, "rb") as f:
        target_height = struct.unpack("<Q", f.read(8))[0]
    
    print(f"-> Storage Chain Height reported by index.dat: {target_height:,}")
    
    # 2. Iterate through chunk directories
    files_per_dir = 65536
    total_chunks = (target_height // files_per_dir) + 1
    
    total_blocks_verified = 0
    total_block_bytes = 0
    total_stmt_bytes = 0
    errors = []
    
    print(f"-> Total Chunks to scan: {total_chunks} (Chunks 00000 to {total_chunks - 1:05d})")
    print("-" * 60)
    
    last_print_time = time.time()
    
    for chunk_id in range(total_chunks):
        chunk_name = f"{chunk_id:05d}"
        chunk_dir = os.path.join(data_dir, chunk_name)
        
        idx_path = os.path.join(chunk_dir, "blocks.idx")
        blocks_path = os.path.join(chunk_dir, "blocks.dat")
        stmts_path = os.path.join(chunk_dir, "statements.dat")
        
        if not os.path.exists(idx_path) or not os.path.exists(blocks_path):
            errors.append(f"Chunk {chunk_name}: Missing blocks.idx or blocks.dat")
            continue
        
        idx_size = os.path.getsize(idx_path)
        blocks_size = os.path.getsize(blocks_path)
        stmts_size = os.path.getsize(stmts_path) if os.path.exists(stmts_path) else 0
        
        total_block_bytes += blocks_size
        total_stmt_bytes += stmts_size
        
        # Calculate expected entries in this chunk
        chunk_start_height = chunk_id * files_per_dir
        if chunk_id == total_chunks - 1:
            expected_entries = (target_height % files_per_dir) + 1
        else:
            expected_entries = files_per_dir
            
        expected_idx_size = expected_entries * 16
        if idx_size < expected_idx_size:
            errors.append(f"Chunk {chunk_name}: idx_size ({idx_size}) < expected ({expected_idx_size})")
        
        # Read index table
        with open(idx_path, "rb") as f_idx:
            idx_bytes = f_idx.read(expected_idx_size)
            
        entries_in_chunk = len(idx_bytes) // 16
        
        for slot in range(entries_in_chunk):
            offset = slot * 16
            b_offset, b_size, s_offset, s_size = struct.unpack("<IIII", idx_bytes[offset:offset+16])
            
            block_height = chunk_start_height + slot
            if block_height == 0:
                continue # slot 0 of chunk 0 is genesis placeholder if 1-indexed
                
            if block_height > target_height:
                break
                
            if b_size == 0 and block_height <= target_height:
                errors.append(f"Block {block_height}: Zero block size at slot {slot}")
            elif b_offset + b_size > blocks_size:
                errors.append(f"Block {block_height}: Block offset {b_offset}+{b_size} exceeds blocks.dat size {blocks_size}")
                
            total_blocks_verified += 1
            
        now = time.time()
        if now - last_print_time >= 3.0 or chunk_id == total_chunks - 1:
            pct = (total_blocks_verified / target_height) * 100
            elapsed = now - start_time
            rate = total_blocks_verified / max(1, elapsed)
            print(f"[{pct:5.1f}%] Scanned Chunk {chunk_name} | Blocks Verified: {total_blocks_verified:,} / {target_height:,} ({rate:,.0f} blk/s)")
            last_print_time = now

    elapsed_total = time.time() - start_time
    
    print("-" * 60)
    print("  AUDIT COMPLETED")
    print(f"  Total Blocks Verified: {total_blocks_verified:,}")
    print(f"  Total Data Size:       {total_block_bytes / (1024**3):.2f} GiB blocks, {total_stmt_bytes / (1024**3):.2f} GiB statements")
    print(f"  Total Time:            {elapsed_total:.2f} seconds ({total_blocks_verified / max(1, elapsed_total):,.0f} blocks/sec)")
    print(f"  Errors Detected:       {len(errors)}")
    
    report = {
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "target_height": target_height,
        "blocks_verified": total_blocks_verified,
        "total_chunks": total_chunks,
        "block_bytes": total_block_bytes,
        "statement_bytes": total_stmt_bytes,
        "elapsed_seconds": elapsed_total,
        "errors": errors,
        "status": "PASS" if len(errors) == 0 else "FAIL"
    }
    
    report_file = os.path.join(data_dir, "audit_report.json")
    with open(report_file, "w") as f:
        json.dump(report, f, indent=2)
    print(f"  Audit Report Saved To: {report_file}")
    print("=" * 60)
    return len(errors) == 0

if __name__ == "__main__":
    data_dir = "/Volumes/SSD/Sirius_data"
    if len(sys.argv) > 1:
        data_dir = sys.argv[1]
    audit_storage(data_dir)
