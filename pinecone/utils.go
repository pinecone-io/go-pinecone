package pinecone

import (
	"bytes"
	"encoding/binary"
	"log"
	"math"
)

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

	return &NdArray{
		Buffer: buf.Bytes(),
		Shape: []uint32{uint32(len(arr)), uint32(len(arr[0]))},
		Dtype: "float32",
	}, nil
}

func FloatArrToNdArrayLogErr(arr [][]float32) *NdArray {
	result, err := FloatArrToNdArray(arr)
	if err != nil {
		log.Fatalf("failed to convert arr; got error: %v", err)
	}
	return result
}

func FloatNdArrayToArr(array *NdArray) ([][]float32, error) {
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

func FloatNdArrayToArrLogErr(array *NdArray) [][]float32 {
	result, err := FloatNdArrayToArr(array)
	if err != nil {
		log.Fatal("failed to convert NdArray; got error: %v", err)
	}
	return result
}

func StringNdArrayToArr(array *NdArray, itemsize int) ([][]string, error) {
	var buf bytes.Buffer
	buf.Write(array.Buffer)

	var vectorCount, vectorDim uint32
	log.Print(array.Shape)
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
			result[i][j] = string(buf.Next(itemsize))
		}
	}
	return result, nil
}

func StringNdArrayToArrLogErr(array *NdArray, itemsize int) [][]string {
	result, err := StringNdArrayToArr(array, itemsize)
	if err != nil {
		log.Fatal("failed to convert NdArray; go error: %v", err)
	}
	return result
}
