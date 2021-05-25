// Package go-pinecone is a go client for Pinecone.io services.
// https://github.com/pinecone-io/go-pinecone
package pinecone

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
)

// FloatArrToNdArray is a utility method that transforms a [][]float32 into an
// NdArray. All rows in arr must have the same length, and arr must be nonempty.
//
// Returns a new NdArray with dtype 'float32', or a non-nil error in case the
// transformation failed.
func FloatArrToNdArray(arr [][]float32) (*NdArray, error) {
	var buf bytes.Buffer

	for i := range arr {
		for j := range arr[i] {
			err := binary.Write(&buf, binary.LittleEndian, arr[i][j])
			if err != nil {
				return nil, err
			}
		}
	}

	var shape []uint32
	if len(arr) == 1 {
		shape = []uint32{uint32(len(arr[0]))}
	} else {
		shape = []uint32{uint32(len(arr)), uint32(len(arr[0]))}
	}

	return &NdArray{
		Buffer: buf.Bytes(),
		Shape:  shape,
		Dtype:  "float32",
	}, nil
}

// FloatNdArrayToArrLogErr is a utility method that transforms an NdArray into a
// [][]float32.
//
// Returns a non-nil error if the transformation fails.
func FloatNdArrayToArr(array *NdArray) ([][]float32, error) {
	if array.Dtype != "float32" {
		return nil, errors.New(fmt.Sprintf("unexpected dtype: %v", array.Dtype))
	}

	var buf bytes.Buffer
	buf.Write(array.Buffer)

	var vectorCount, vectorDim uint32
	if len(array.Shape) == 1 {
		vectorCount, vectorDim = 1, array.Shape[0]
	} else {
		vectorCount, vectorDim = array.Shape[0], array.Shape[1]
	}

	result := make([][]float32, vectorCount)
	for i := range result {
		result[i] = make([]float32, vectorDim)
	}

	for i := range result {
		for j := range result[i] {
			bits := binary.LittleEndian.Uint32(buf.Next(4))
			result[i][j] = math.Float32frombits(bits)
		}
	}
	return result, nil
}

// StringNdArrayToArrLogErr is a utility method that transforms an NdArray into a
// [][]string.
//
// Returns a non-nil error if the transformation fails.
func StringNdArrayToArr(array *NdArray) ([][]string, error) {
	if array.Dtype[:2] != "|S" {
		return nil, errors.New(fmt.Sprintf("unexpected dtype: %v", array.Dtype[:2]))
	}

	itemSize, err := strconv.ParseInt(array.Dtype[2:], 10, 32)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(array.Buffer)

	var vectorCount, vectorDim uint32
	if len(array.Shape) == 1 {
		vectorCount, vectorDim = 1, array.Shape[0]
	} else {
		vectorCount, vectorDim = array.Shape[0], array.Shape[1]
	}

	result := make([][]string, vectorCount)
	for i := range result {
		result[i] = make([]string, vectorDim)
	}

	for i := range result {
		for j := range result[i] {
			result[i][j] = string(bytes.Trim(buf.Next(int(itemSize)), "\x00"))
		}
	}
	return result, nil
}
