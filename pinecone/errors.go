package pinecone

import (
	"fmt"
	"maps"
)

// PineconeError is the error returned when a Pinecone REST API request (control plane, admin,
// inference, or REST data plane) receives a non-success HTTP response. It is returned as a
// *PineconeError; use errors.As to read Code, ErrorCode and Reason.
//
// Data-plane calls made over gRPC (vector upsert, query, fetch, update, delete, stats, and namespace
// operations) return gRPC status errors instead; inspect them with status.Code(err) from
// google.golang.org/grpc/status. Network and response-decoding failures are returned as plain
// errors.
type PineconeError struct {
	// Code is the HTTP status code of the response.
	Code int
	// ErrorCode is the API's error code from the response body, such as "ABORTED" when a read
	// capacity change is still in progress, or "ALREADY_EXISTS". It's empty when the body isn't
	// an API error response. Branch on it rather than on the message text.
	ErrorCode string
	// Reason is the exact cause of the error, such as "TOO_MANY_DOCUMENTS", when the API names one.
	// Only some errors set it, and only on API version 2026-10 and later. Each reason belongs to one
	// ErrorCode. New reasons may be added: treat a value you don't recognize as if Reason were
	// empty, and act on ErrorCode.
	Reason string
	// Message is the error message from the response body, prefixed with the operation that
	// failed. It's empty when the body isn't an API error response.
	Message string
	// Msg holds a JSON-encoded summary of the response: the status code, the raw body, and the
	// error code, reason, message, and details decoded from the body when available.
	Msg error

	// details is a pointer so that PineconeError stays comparable with ==.
	details *map[string]any
}

// ReasonResourceIncompatibleWithAPIVersion is the [PineconeError.Reason] the API sets, with the
// error code FAILED_PRECONDITION and HTTP status 412, when a call names an index, collection,
// backup, or restore job that uses features this SDK's API version can't represent. The resource's
// entry in [Client.ListIncompatibleIndexes], [Client.ListIncompatibleCollections],
// [BackupList].IncompatibleBackups, or [RestoreJobList].IncompatibleRestoreJobs names an API version
// that can describe it.
const ReasonResourceIncompatibleWithAPIVersion = "RESOURCE_INCOMPATIBLE_WITH_API_VERSION"

// Details returns a copy of the machine-readable facts about the error, keyed as its Reason
// documents, or nil when the response has none. Values are strings, numbers or booleans; numbers
// decode as float64, as with encoding/json.
func (pe *PineconeError) Details() map[string]any {
	if pe == nil || pe.details == nil {
		return nil
	}
	return maps.Clone(*pe.details)
}

// Error returns the formatted Msg.
func (pe *PineconeError) Error() string {
	return fmt.Sprintf("%+v", pe.Msg)
}
