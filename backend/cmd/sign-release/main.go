package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "keygen":
		cmdKeygen(os.Args[2:])
	case "sign":
		cmdSign(os.Args[2:])
	case "verify":
		cmdVerify(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`ProximaX Sirius Release Signing & Integrity Utility

Usage:
  sign-release keygen [-out-dir <dir>]
      Generate a new Ed25519 release signing keypair.

  sign-release sign -key <privKeyHex|path> -dir <releaseDir> [-out <checksumsFile>]
      Compute SHA256SUMS for all files in releaseDir and generate an Ed25519 signature (.sig).

  sign-release verify -pubkey <pubKeyHex|path> -checksums <checksumsFile> -sig <sigFile> [-dir <releaseDir>]
      Verify the cryptographic signature of SHA256SUMS and validate file checksums.`)
}

func cmdKeygen(args []string) {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	outDir := fs.String("out-dir", ".", "Directory to output keys")
	_ = fs.Parse(args)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key: %v\n", err)
		os.Exit(1)
	}

	privHex := hex.EncodeToString(priv)
	pubHex := hex.EncodeToString(pub)

	privPath := filepath.Join(*outDir, "release-ed25519.key")
	pubPath := filepath.Join(*outDir, "release-ed25519.pub")

	// Write private key with restrictive 0600 permissions
	if err := os.WriteFile(privPath, []byte(privHex+"\n"), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing private key: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(pubPath, []byte(pubHex+"\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing public key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Generated new Ed25519 release keypair:\n")
	fmt.Printf("  Private Key File (SECRET): %s (chmod 0600)\n", privPath)
	fmt.Printf("  Public Key File:           %s\n", pubPath)
	fmt.Printf("  Public Key (Hex):          %s\n", pubHex)
}

func cmdSign(args []string) {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	keyArg := fs.String("key", "", "64-byte Ed25519 private key hex string or path to key file")
	releaseDir := fs.String("dir", "", "Directory containing release artifacts to hash and sign")
	outFile := fs.String("out", "", "Output path for checksums (default: <dir>/SHA256SUMS)")
	_ = fs.Parse(args)

	if *keyArg == "" || *releaseDir == "" {
		fmt.Fprintln(os.Stderr, "Error: -key and -dir are required")
		fs.Usage()
		os.Exit(1)
	}

	privKeyBytes, err := resolvePrivateKey(*keyArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading private key: %v\n", err)
		os.Exit(1)
	}

	entries, err := os.ReadDir(*releaseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading release directory: %v\n", err)
		os.Exit(1)
	}

	var checksumLines []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "SHA256SUMS" || strings.HasSuffix(name, ".sig") || strings.HasPrefix(name, ".") {
			continue
		}

		filePath := filepath.Join(*releaseDir, name)
		hash, err := computeFileSHA256(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error hashing %s: %v\n", name, err)
			os.Exit(1)
		}
		checksumLines = append(checksumLines, fmt.Sprintf("%s  %s", hash, name))
	}

	sort.Strings(checksumLines)
	if len(checksumLines) == 0 {
		fmt.Fprintln(os.Stderr, "Warning: No release artifact files found in directory to sign.")
	}

	checksumsContent := strings.Join(checksumLines, "\n") + "\n"

	targetChecksumFile := *outFile
	if targetChecksumFile == "" {
		targetChecksumFile = filepath.Join(*releaseDir, "SHA256SUMS")
	}

	if err := os.WriteFile(targetChecksumFile, []byte(checksumsContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", targetChecksumFile, err)
		os.Exit(1)
	}

	sigBytes := ed25519.Sign(privKeyBytes, []byte(checksumsContent))
	targetSigFile := targetChecksumFile + ".sig"
	if err := os.WriteFile(targetSigFile, sigBytes, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", targetSigFile, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Successfully computed checksums and Ed25519 signature:\n")
	fmt.Printf("  Checksums: %s (%d files)\n", targetChecksumFile, len(checksumLines))
	fmt.Printf("  Signature: %s (%d bytes)\n", targetSigFile, len(sigBytes))
}

func cmdVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	pubKeyArg := fs.String("pubkey", "", "32-byte Ed25519 public key hex string or path to pub key file")
	checksumsArg := fs.String("checksums", "", "Path to SHA256SUMS file")
	sigArg := fs.String("sig", "", "Path to SHA256SUMS.sig file")
	releaseDir := fs.String("dir", "", "Optional: directory containing artifacts to check against SHA256SUMS")
	_ = fs.Parse(args)

	if *pubKeyArg == "" || *checksumsArg == "" || *sigArg == "" {
		fmt.Fprintln(os.Stderr, "Error: -pubkey, -checksums, and -sig are required")
		fs.Usage()
		os.Exit(1)
	}

	pubKeyBytes, err := resolvePublicKey(*pubKeyArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading public key: %v\n", err)
		os.Exit(1)
	}

	checksumsData, err := os.ReadFile(*checksumsArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading checksums file: %v\n", err)
		os.Exit(1)
	}

	sigData, err := os.ReadFile(*sigArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading signature file: %v\n", err)
		os.Exit(1)
	}

	// 1. Verify Ed25519 signature over checksums content
	if !ed25519.Verify(pubKeyBytes, checksumsData, sigData) {
		fmt.Fprintln(os.Stderr, "✕ Cryptographic signature verification FAILED! Data may be forged or corrupted.")
		os.Exit(1)
	}
	fmt.Println("✓ Ed25519 Cryptographic Signature Verified: SHA256SUMS is authentic!")

	// 2. If releaseDir provided, verify each file's sha256
	if *releaseDir != "" {
		lines := strings.Split(string(checksumsData), "\n")
		verifiedCount := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			expectedHash := strings.ToLower(parts[0])
			fileName := filepath.Base(parts[1])
			filePath := filepath.Join(*releaseDir, fileName)

			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				fmt.Printf("  ⚠ Warning: file not found in dir: %s\n", fileName)
				continue
			}

			actualHash, err := computeFileSHA256(filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "✕ Error computing hash for %s: %v\n", fileName, err)
				os.Exit(1)
			}

			if actualHash != expectedHash {
				fmt.Fprintf(os.Stderr, "✕ CHECKSUM MISMATCH for %s!\n  Expected: %s\n  Actual:   %s\n", fileName, expectedHash, actualHash)
				os.Exit(1)
			}
			fmt.Printf("  ✓ File OK: %s (%s)\n", fileName, actualHash[:16]+"...")
			verifiedCount++
		}
		fmt.Printf("✓ All %d file checksums verified successfully!\n", verifiedCount)
	}
}

func computeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func resolvePrivateKey(input string) (ed25519.PrivateKey, error) {
	str := strings.TrimSpace(input)
	if data, err := os.ReadFile(str); err == nil {
		str = strings.TrimSpace(string(data))
	}

	bytes, err := hex.DecodeString(str)
	if err != nil {
		return nil, fmt.Errorf("invalid hex encoding: %w", err)
	}

	if len(bytes) == ed25519.PrivateKeySize {
		return ed25519.PrivateKey(bytes), nil
	} else if len(bytes) == 32 {
		// Seed format (32 bytes) -> expand to private key
		return ed25519.NewKeyFromSeed(bytes), nil
	}

	return nil, fmt.Errorf("invalid private key length: got %d bytes, expected 32 or 64", len(bytes))
}

func resolvePublicKey(input string) (ed25519.PublicKey, error) {
	str := strings.TrimSpace(input)
	if data, err := os.ReadFile(str); err == nil {
		str = strings.TrimSpace(string(data))
	}

	bytes, err := hex.DecodeString(str)
	if err != nil {
		return nil, fmt.Errorf("invalid hex encoding: %w", err)
	}

	if len(bytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key length: got %d bytes, expected 32", len(bytes))
	}

	return ed25519.PublicKey(bytes), nil
}
