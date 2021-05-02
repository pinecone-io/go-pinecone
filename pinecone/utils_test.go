package pinecone

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFloatArrToNdArray_sanity(t *testing.T) {
	assert := assert.New(t)

	arr := [][]float32{
		{3.14, 1.59, 2.65},
	}

	result, err := FloatArrToNdArray(arr)
	assert.Nil(err,"FloatArrToNdArray returned an unexpected error.")

	expectedResult := NdArray{
		Buffer:     []byte("\xc3\xf5H@\x1f\x85\xcb?\x9a\x99)@"),
		Shape:      []uint32{3},
		Dtype:      "float32",
		Compressed: false,
	}
	assert.Equal(&expectedResult, result, "FloatArrToNdArray returned an unexpected result.")
}

func TestFloatNdArrayToArr_sanity(t *testing.T) {
	assert := assert.New(t)

	ndArray := NdArray{
		Buffer:     []byte("\xc3\xf5H@\x1f\x85\xcb?\x9a\x99)@"),
		Shape:      []uint32{3},
		Dtype:      "float32",
		Compressed: false,
	}

	result, err := FloatNdArrayToArr(&ndArray)
	assert.Nil(err,"FloatNdArrayToArr returned an unexpected error.")

	expectedResult := [][]float32{
		{3.14, 1.59, 2.65},
	}
	assert.Equal(expectedResult, result,
		"FloatNdArrayToArr returned an unexpected result.")
}

func TestStringNdArrayToArr_sanity(t *testing.T) {
	assert := assert.New(t)

	ndArray := NdArray{
		Buffer:     []byte("string1\x00\x00\x00\x00\x00\x00\x00string2\x00\x00\x00\x00\x00\x00\x00another string"),
		Shape:      []uint32{3},
		Dtype:      "|S14",
		Compressed: false,
	}

	result, err := StringNdArrayToArr(&ndArray)
	assert.Nil(err,"StringNdArrayToArr returned an unexpected result.")

	expectedResult := [][]string{
		{"string1", "string2", "another string"},
	}
	assert.Equal(expectedResult, result,
		"StringNdArrayToArr returned an unexpected result.")
}
