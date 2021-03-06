package handel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var nb = NewWilffBitset

type bitsetTest struct {
	fb          func() BitSet
	bitlength   int
	cardinality int
	setBits     []int
}

func TestBitSetWilff(t *testing.T) {
	var tests = []bitsetTest{
		{func() BitSet { return nb(10) }, 10, 0, []int{}},
		{
			func() BitSet {
				b := nb(10)
				b.Set(0, true)
				b.Set(1, true)
				return b
			}, 10, 2, []int{0, 1},
		},
		{
			func() BitSet {
				b := nb(10)
				b.Set(11, true)
				b.Set(3, true)
				return b
			}, 10, 1, []int{3},
		},
	}
	testBitSets(t, tests)
}

func testBitSets(t *testing.T, tests []bitsetTest) {
	for _, tt := range tests {
		bitset := tt.fb()
		require.Equal(t, tt.bitlength, bitset.BitLength())
		require.Equal(t, tt.cardinality, bitset.Cardinality())
		for _, idx := range tt.setBits {
			require.True(t, bitset.Get(idx))
		}
	}

}

func testBitSetWilffMarshalling(t *testing.T) {
	b := NewWilffBitset(10).(*WilffBitSet)
	b.Set(1, true)
	b.Set(4, true)
	buff, err := b.MarshalBinary()
	require.NoError(t, err)

	b2 := new(WilffBitSet)
	err = b2.UnmarshalBinary(buff)
	require.NoError(t, err)

	require.Equal(t, b.l, b2.l)
}
