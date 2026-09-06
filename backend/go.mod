module proximax-sirius-core

go 1.24

require (
	github.com/gorilla/websocket v1.5.3
	github.com/klauspost/compress v1.19.2
	github.com/proximax-storage/go-xpx-chain-sdk v0.8.4
	github.com/proximax-storage/go-xpx-crypto v0.1.0
	golang.org/x/crypto v0.14.0
)

require (
	github.com/google/flatbuffers v1.11.0 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/proximax-storage/go-xpx-utils v0.0.0-20190604083640-90d06ff8a19f // indirect
	github.com/supranational/blst v0.3.18-0.20260818190240-de54cd4684a3 // indirect
	golang.org/x/sys v0.13.0 // indirect
)

// Security Invariant: Local fork of go-xpx-crypto with exported Destroy()/Zero() methods
// on PrivateKey and KeyPair to guarantee explicit RAM memory wiping of both the Raw []byte
// slice and the unexported math/big.Int words upon transaction signing completion.
// See internal/crypto/PATCHES.md for full details before making any dependency upgrades.
replace github.com/proximax-storage/go-xpx-crypto => ./internal/crypto
