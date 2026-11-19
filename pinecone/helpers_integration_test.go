//go:build integration || localServer

package pinecone

import (
	"fmt"
	"math/rand"
	"testing"
)

// blockedSkipPrefix starts every skip that waits on something outside the SDK. CI counts these
// skips and fails a pull request into main that has any.
const blockedSkipPrefix = "BLOCKED: "

// skipBlocked skips t until blocker, a plain-words description of what has to ship first, clears.
func skipBlocked(t testing.TB, blocker string) {
	t.Helper()
	t.Skip(blockedSkipPrefix + blocker)
}

func generateVectors(numOfVectors int, dimension int32, isSparse bool, metadata *Metadata) []*Vector {
	vectors := make([]*Vector, numOfVectors)

	for i := 0; i < int(numOfVectors); i++ {
		vectors[i] = &Vector{
			Id: fmt.Sprintf("vector-%d", i),
		}

		if isSparse {
			var sparseValues SparseValues
			for j := 0; j < int(dimension); j++ {
				sparseValues.Indices = append(sparseValues.Indices, uint32(j))
			}
			values := generateVectorValues(dimension)
			sparseValues.Values = *values
			vectors[i].SparseValues = &sparseValues
		} else {
			values := generateVectorValues(dimension)
			vectors[i].Values = values
		}

		if metadata != nil {
			vectors[i].Metadata = metadata
		}
	}

	return vectors
}

func generateVectorValues(dimension int32) *[]float32 {
	maxInt := 1000000 // A large integer to normalize the float values
	values := make([]float32, dimension)

	for i := int32(0); i < dimension; i++ {
		// Generate a random integer and normalize it to the range [0, 1)
		values[i] = float32(rand.Intn(maxInt)) / float32(maxInt)
	}

	return &values
}

func checkEgressBytes[T uint32 | int64](egressBytes *T) error {
	if egressBytes == nil {
		return fmt.Errorf("usage.egress_bytes is missing")
	}
	if *egressBytes <= 0 {
		return fmt.Errorf("usage.egress_bytes is %d, want > 0", *egressBytes)
	}
	return nil
}
