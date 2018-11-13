package handel

import (
	"errors"
	"fmt"
	"sync"
)

// Handel is the principal struct that performs the large scale multi-signature
// aggregation protocol. Handel is thread-safe.
type Handel struct {
	sync.Mutex
	// Config holding parameters to Handel
	c *Config
	// Network enabling external communication with other Handel nodes
	net Network
	// Registry holding access to all Handel node's identities
	reg Registry
	// signature scheme used for this Handel protocol
	scheme SignatureScheme
	// Message that is being signed during the Handel protocol
	msg []byte
	// candidate tree helper to select nodes at each level
	tree *candidateTree
	// incremental aggregated signature cached by this handel node
	aggregate Signature
	// incremental level at which this handel node is at
	level uint16
	// level at which this handel node is at
	// channel to exposes multi-signatures to the user
	out chan MultiSignature
}

// NewHandel returns a Handle interface that uses the given network and
// registry. The signature scheme is the one to use for this Handel protocol,
// and the message is the message to multi-sign.The first config in the slice is
// taken if not nil. Otherwise, the default config generated by DefaultConfig()
// is used.
func NewHandel(n Network, r Registry, id Identity, s SignatureScheme, msg []byte,
	conf ...*Config) (*Handel, error) {
	h := &Handel{
		net:    n,
		reg:    r,
		tree:   newCandidateTree(id.ID(), r),
		scheme: s,
		msg:    msg,
	}

	if len(conf) > 0 && conf[0] != nil {
		h.c = mergeWithDefault(conf[0], r.Size())
	} else {
		h.c = DefaultConfig(r.Size())
	}

	ms, err := s.Sign(msg, nil)
	if err != nil {
		return nil, err
	}
	h.aggregate = ms
	return h, nil
}

// NewPacket implements the Listener interface for the network.
// It returns an error in case the packet is not a properly formatted packet or
// contains erroneous data.
func (h *Handel) NewPacket(p *Packet) error {
	h.Lock()
	defer h.Unlock()

	_, err := h.parsePacket(p)
	if err != nil {
		return err
	}
	return nil
}

// Start the Handel protocol
func (h *Handel) Start() {
	h.Lock()
	defer h.Unlock()
}

// parsePacket returns the multisignature parsed from the given packet, or an
// error if the packet can't be unmarshalled, or contains erroneous data such as
// an invalid signature or out of range origin. This method is NOT thread-safe
// and only meant for internal use.
func (h *Handel) parsePacket(p *Packet) (*MultiSignature, error) {
	if p.Origin >= uint32(h.reg.Size()) {
		return nil, errors.New("handel: packet's origin out of range")
	}

	if int(p.Level) > log2(h.reg.Size()) {
		return nil, errors.New("handel: packet's level out of range")
	}

	ms := new(MultiSignature)
	err := ms.Unmarshal(p.MultiSig, h.scheme.Signature(), h.c.NewBitSet())
	if err != nil {
		return nil, err
	}

	err = h.verifySignature(ms, p.Origin, p.Level)
	return ms, err
}

func (h *Handel) verifySignature(ms *MultiSignature, origin uint32, level byte) error {
	// check inclusion of sender within the given range
	min, max, err := h.tree.RangeAt(int(level))
	if err != nil {
		return err
	}
	if int(origin) < min || int(origin) >= max {
		return errors.New("handel: origin not corresponding to level's range")
	}
	if ms.BitSet.BitLength() != (max - min) {
		return errors.New("handel: inconsistent bitset with given level")
	}

	// compute the aggregate public key corresponding to bitset
	ids, ok := h.reg.Identities(min, max)
	if !ok {
		return errors.New("handel: identities can't be retrieved from given range")
	}
	aggregateKey := h.scheme.PublicKey()
	for i := 0; i < ms.BitSet.BitLength(); i++ {
		if !ms.BitSet.Get(i) {
			continue
		}
		aggregateKey = aggregateKey.Combine(ids[i].PublicKey())
	}

	if err := aggregateKey.VerifySignature(h.msg, ms.Signature); err != nil {
		return fmt.Errorf("handel: %s", err)
	}
	return nil
}
