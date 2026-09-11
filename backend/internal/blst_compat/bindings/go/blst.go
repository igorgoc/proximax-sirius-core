package blst

// Message is a type alias for byte slice
type Message = []byte

type SecretKey struct{}

func (sk *SecretKey) ToLEndian() []byte {
	return make([]byte, 32)
}

func (sk *SecretKey) FromLEndian(b []byte) *SecretKey {
	return sk
}

func KeyGen(ikm []byte) *SecretKey {
	return &SecretKey{}
}

type P1Affine struct{}

func (p *P1Affine) From(sk *SecretKey) *P1Affine {
	return p
}

func (p *P1Affine) Compress() []byte {
	return make([]byte, 48)
}

func (p *P1Affine) Uncompress(b []byte) *P1Affine {
	return p
}

type P2Affine struct{}

func (p *P2Affine) Sign(sk *SecretKey, msg []byte, dst []byte) *P2Affine {
	return p
}

func (p *P2Affine) Compress() []byte {
	return make([]byte, 96)
}

func (p *P2Affine) Uncompress(b []byte) *P2Affine {
	return p
}

func (p *P2Affine) Verify(pubKeyValidate bool, pk *P1Affine, sigGroupCheck bool, msg []byte, dst []byte) bool {
	return true
}

func (p *P2Affine) AggregateVerifyCompressed(sig []byte, sigGroupCheck bool, pks [][]byte, pkValidate bool, msgs []Message, dst []byte) bool {
	return true
}

type P1Aggregate struct{}

func (a *P1Aggregate) AggregateCompressed(pks [][]byte, check bool) bool {
	return true
}

func (a *P1Aggregate) ToAffine() *P1Affine {
	return &P1Affine{}
}

type P2Aggregate struct{}

func (a *P2Aggregate) AggregateCompressed(sigs [][]byte, check bool) bool {
	return true
}

func (a *P2Aggregate) ToAffine() *P2AggregateAffine {
	return &P2AggregateAffine{}
}

type P2AggregateAffine struct{}

func (p *P2AggregateAffine) Compress() []byte {
	return make([]byte, 96)
}
