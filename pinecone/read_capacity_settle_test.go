package pinecone

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readCapacitySettled reports whether the index's dedicated read capacity is Ready. It returns an
// error for the Error state, so callers stop waiting instead of polling until the deadline.
func readCapacitySettled(idx *Index) (bool, error) {
	rc := idx.ReadCapacity
	if rc == nil || rc.Dedicated == nil {
		return false, nil
	}
	switch rc.Dedicated.Status.State {
	case "Ready":
		return true, nil
	case "Error":
		msg := "no error message"
		if rc.Dedicated.Status.ErrorMessage != nil {
			msg = *rc.Dedicated.Status.ErrorMessage
		}
		return false, fmt.Errorf("read capacity is in the Error state: %s", msg)
	default:
		return false, nil
	}
}

func TestReadCapacitySettledUnit(t *testing.T) {
	dedicated := func(state string, errorMessage *string) *Index {
		return &Index{ReadCapacity: &ReadCapacity{Dedicated: &ReadCapacityDedicated{
			Status: ReadCapacityStatus{State: state, ErrorMessage: errorMessage},
		}}}
	}

	settled, err := readCapacitySettled(dedicated("Ready", nil))
	require.NoError(t, err)
	assert.True(t, settled)

	for _, state := range []string{"Scaling", "Migrating", "Burst"} {
		settled, err = readCapacitySettled(dedicated(state, nil))
		require.NoError(t, err, state)
		assert.False(t, settled, state)
	}

	settled, err = readCapacitySettled(&Index{})
	require.NoError(t, err)
	assert.False(t, settled)

	_, err = readCapacitySettled(dedicated("Error", ptr("not enough capacity")))
	require.ErrorContains(t, err, "not enough capacity")
	_, err = readCapacitySettled(dedicated("Error", nil))
	require.ErrorContains(t, err, "Error state")
}
