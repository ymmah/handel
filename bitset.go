package handel

import (
	"bytes"
	"encoding/binary"

	"github.com/willf/bitset"
)

// BitSet is a bitset !
type BitSet interface {
	// BitLength returns the fixed size of this BitSet
	BitLength() int
	// Cardinality returns the number of '1''s set
	Cardinality() int
	// Set the bit at the given index to 1 or 0 depending on the given boolean.
	// If the index is out of bound, implementations MUST not change the bitset.
	Set(int, bool)
	// Get returns the status of the i-th bit in this bitset. Implementations
	// must return false if the index is out of bounds.
	Get(int) bool
	// Slice returns a BitSet that only contains the bits between the given
	// range, to excluded. If the range given is invalid, it returns the same
	// bitset.
	Slice(from, to int) BitSet
	// MarshalBinary returns the binary representation of the BitSet.
	MarshalBinary() ([]byte, error)
	// UnmarshalBinary fills the bitset from the given buffer.
	UnmarshalBinary([]byte) error
	// returns the binary representation of this bitset in string
	String() string
}

// WilffBitSet implementats a BitSet using the wilff library.
type WilffBitSet struct {
	b *bitset.BitSet
	l int
}

// NewWilffBitset returns a BitSet implemented using the wilff's bitset library.
func NewWilffBitset(length int) BitSet {
	return &WilffBitSet{
		b: bitset.New(uint(length)),
		l: length,
	}
}

// BitLength implements the BitSet interface
func (w *WilffBitSet) BitLength() int {
	return w.l
}

// Cardinality implements the BitSet interface
func (w *WilffBitSet) Cardinality() int {
	return int(w.b.Count())
}

// Set implements the BitSet interface
func (w *WilffBitSet) Set(idx int, status bool) {
	if !w.inBound(idx) {
		// do nothing if out of bounds
		return
	}
	w.b = w.b.SetTo(uint(idx), status)
}

// Get implements the BitSet interface
func (w *WilffBitSet) Get(idx int) bool {
	if !w.inBound(idx) {
		return false
	}
	return w.b.Test(uint(idx))
}

// Combine implements the BitSet interface
func (w *WilffBitSet) Combine(b2 BitSet) BitSet {
	// XXX Panics if used wrongly at the moment. Could be possible to use other
	// implementations by using the interface's method and implementing or
	// ourselves.
	w2 := b2.(*WilffBitSet)
	totalLength := w.l + w2.l
	w3 := NewWilffBitset(totalLength)
	for i := 0; i < w.l; i++ {
		w3.Set(i, w.Get(i))
	}
	for i := 0; i < w2.l; i++ {
		w3.Set(i+w.l, w2.Get(i))
	}
	return w
}

// Slice implements the BitSet interface
func (w *WilffBitSet) Slice(from, to int) BitSet {
	if !w.inBound(from) || to < from || to > w.l {
		return w
	}
	newLength := to - from
	w2 := NewWilffBitset(newLength)
	for i := 0; i < newLength; i++ {
		w2.Set(i, w2.Get(i+from))
	}
	return w2
}

func (w *WilffBitSet) inBound(idx int) bool {
	return !(idx < 0 || idx >= w.l)
}

// MarshalBinary implements the go Marshaler interface. It encodes the size
// first and then the bitset.
func (w *WilffBitSet) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	err := binary.Write(&b, binary.BigEndian, uint16(w.l))
	if err != nil {
		return nil, err
	}
	buff, err := w.b.MarshalBinary()
	if err != nil {
		return nil, err
	}
	b.Write(buff)
	return b.Bytes(), nil
}

// UnmarshalBinary implements the go Marshaler interface. It decodes the length
// first and then the bitset.
func (w *WilffBitSet) UnmarshalBinary(buff []byte) error {
	var b = bytes.NewBuffer(buff)
	var length uint16
	err := binary.Read(b, binary.BigEndian, &length)
	if err != nil {
		return err
	}

	w.b = new(bitset.BitSet)
	w.l = int(length)
	return w.b.UnmarshalBinary(b.Bytes())
}

func (w *WilffBitSet) String() string {
	return w.b.String()
}
