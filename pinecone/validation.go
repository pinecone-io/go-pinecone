package pinecone

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
)

// Client-side bounds matching what the server enforces. Rejecting locally means a call the server
// was going to refuse fails before any request is sent. These are deployment settings rather than
// constants of the API: a deployment configured lower can still reject values these checks let
// through.
const (
	minTopK                 = 1
	maxTopK                 = 10000
	maxVectorIdLength       = 512
	minListLimit            = 1
	maxListLimit            = 100
	maxFetchByMetadataLimit = 10000
)

// validateTopK bounds query top_k at both ends before any request is made.
func validateTopK(topK uint32) error {
	if topK < minTopK || topK > maxTopK {
		return fmt.Errorf("TopK must be between %d and %d, got %d", minTopK, maxTopK, topK)
	}
	return nil
}

// validateVectorId enforces the shared vector ID / prefix shape: 1-512 characters with no NUL byte.
func validateVectorId(label string, id string) error {
	if id == "" {
		return fmt.Errorf("%s must not be empty", label)
	}
	if len(id) > maxVectorIdLength {
		return fmt.Errorf("%s exceeds the maximum length of %d characters, got %d", label, maxVectorIdLength, len(id))
	}
	if strings.ContainsRune(id, 0) {
		return fmt.Errorf("%s must not contain a NUL byte", label)
	}
	return nil
}

// validateNonEmptyFilter rejects a metadata filter with no conditions locally. The server has always
// refused an empty filter; delete is the one operation with a true match-everything mode, spelled
// DeleteAllVectorsInNamespace rather than an empty filter.
func validateNonEmptyFilter(filter *MetadataFilter) error {
	if filter == nil || len(filter.Fields) == 0 {
		return fmt.Errorf("filter must contain at least one condition")
	}
	return nil
}

// validateMetadata checks each metadata value before the request is sent: a value must be a string,
// number, boolean, or list of strings. Null values are accepted; the server strips them on write
// rather than refusing them.
func validateMetadata(metadata *Metadata) error {
	if metadata == nil {
		return nil
	}
	for field, value := range metadata.Fields {
		if err := validateMetadataValue(field, value); err != nil {
			return err
		}
	}
	return nil
}

func validateMetadataValue(field string, value *structpb.Value) error {
	switch kind := value.GetKind().(type) {
	case *structpb.Value_StringValue, *structpb.Value_NumberValue, *structpb.Value_BoolValue, *structpb.Value_NullValue, nil:
		return nil
	case *structpb.Value_ListValue:
		for _, entry := range kind.ListValue.GetValues() {
			if _, ok := entry.GetKind().(*structpb.Value_StringValue); !ok {
				return fmt.Errorf("metadata value must be a string, number, boolean or list of strings for field %q", field)
			}
		}
		return nil
	default:
		return fmt.Errorf("metadata value must be a string, number, boolean or list of strings for field %q", field)
	}
}
