package pinecone

import (
	"fmt"
	"math"
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
	// an API error response. Branch on it rather than on the message text. Bulk import errors
	// carry a numeric gRPC status code, which ErrorCode holds by its canonical name, such as
	// "NOT_FOUND" for code 5; a code the SDK doesn't recognize is "UNKNOWN".
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
// backup, or restore job that uses features this SDK's API version can't represent. Retrying with
// the same API version fails.
//
// [PineconeError.Details], when the API sends details, holds "api_version", the API version of the
// request, and the string keys "minimum_api_version" and "preview_api_version", which mean the same
// as MinimumAPIVersion and PreviewAPIVersion on [IncompatibleIndex]. A version key is left out when
// there's no such version; when both are left out, no public API version can act on the resource
// yet. The resource's entry in [Client.ListIncompatibleIndexes],
// [Client.ListIncompatibleCollections], [BackupList].IncompatibleData, or
// [RestoreJobList].IncompatibleData carries the same versions. [PineconeError.TypedDetails]
// reads the details keys as typed fields.
const ReasonResourceIncompatibleWithAPIVersion = "RESOURCE_INCOMPATIBLE_WITH_API_VERSION"

// Reasons the 2026-10 API documents for [PineconeError.Reason]. New reasons may be added: treat a
// value you don't recognize as if Reason were empty, and act on ErrorCode.
const (
	// ReasonTooManyDocuments is the Reason, with the error code INVALID_ARGUMENT, when a documents
	// request has more documents than the operation takes. [ErrorDetails].LimitDocuments and
	// ActualDocuments give the limit and the count sent.
	ReasonTooManyDocuments = "TOO_MANY_DOCUMENTS"
	// ReasonRateLimitExceeded is the Reason, with the error code RESOURCE_EXHAUSTED, when a
	// per-second or per-minute limit was hit. The same request can succeed after backing off. Today
	// embedding limits set it, with [ErrorDetails].Field and Model.
	ReasonRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
	// ReasonMonthlyLimitExceeded is the Reason, with the error code RESOURCE_EXHAUSTED, when a limit
	// for the current month was hit. Retrying fails until the limit resets or the plan changes. Today
	// embedding limits set it, with [ErrorDetails].Field and Model.
	ReasonMonthlyLimitExceeded = "MONTHLY_LIMIT_EXCEEDED"
)

// ErrorDetails is a typed view of the keys the API documents for [PineconeError.Details]. Each
// field is nil when its key is absent or its value isn't of the documented type. Which keys are set
// depends on the Reason.
type ErrorDetails struct {
	// APIVersion is "api_version", the API version of the request. Set with
	// [ReasonResourceIncompatibleWithAPIVersion].
	APIVersion *string
	// MinimumAPIVersion is "minimum_api_version", the earliest generally available API version
	// that can act on the resource; every later one can too. Set with
	// [ReasonResourceIncompatibleWithAPIVersion] when there is such a version.
	MinimumAPIVersion *string
	// PreviewAPIVersion is "preview_api_version", the earliest early-access API version that can
	// act on the resource. Set with [ReasonResourceIncompatibleWithAPIVersion] when there is such a
	// version.
	PreviewAPIVersion *string
	// LimitDocuments is "limit_documents", the most documents one request takes. Set with
	// [ReasonTooManyDocuments].
	LimitDocuments *int
	// ActualDocuments is "actual_documents", how many documents the request sent. Set with
	// [ReasonTooManyDocuments].
	ActualDocuments *int
	// Field is "field", the name of the field being embedded. Set with [ReasonRateLimitExceeded]
	// and [ReasonMonthlyLimitExceeded] for embedding limits.
	Field *string
	// Model is "model", the name of the embedding model. Set with [ReasonRateLimitExceeded] and
	// [ReasonMonthlyLimitExceeded] for embedding limits.
	Model *string
}

// TypedDetails returns the documented keys of [PineconeError.Details] as an [ErrorDetails], or nil
// when the response has no details. Use Details for keys ErrorDetails doesn't name.
//
// Example:
//
//	var pcErr *pinecone.PineconeError
//	if errors.As(err, &pcErr) && pcErr.Reason == pinecone.ReasonResourceIncompatibleWithAPIVersion {
//		if d := pcErr.TypedDetails(); d != nil && d.MinimumAPIVersion != nil {
//			fmt.Println("use API version", *d.MinimumAPIVersion, "or later")
//		}
//	}
func (pe *PineconeError) TypedDetails() *ErrorDetails {
	if pe == nil || pe.details == nil {
		return nil
	}
	d := *pe.details
	return &ErrorDetails{
		APIVersion:        detailString(d, "api_version"),
		MinimumAPIVersion: detailString(d, "minimum_api_version"),
		PreviewAPIVersion: detailString(d, "preview_api_version"),
		LimitDocuments:    detailInt(d, "limit_documents"),
		ActualDocuments:   detailInt(d, "actual_documents"),
		Field:             detailString(d, "field"),
		Model:             detailString(d, "model"),
	}
}

func detailString(d map[string]any, key string) *string {
	if v, ok := d[key].(string); ok {
		return &v
	}
	return nil
}

func detailInt(d map[string]any, key string) *int {
	v, ok := d[key].(float64)
	if !ok || v != math.Trunc(v) || math.Abs(v) > 1<<53 {
		return nil
	}
	i := int(v)
	return &i
}

// Details returns a copy of the machine-readable facts about the error, keyed as its Reason
// documents, or nil when the response has none. Values are decoded as with encoding/json: strings,
// float64 numbers, booleans, or nested map[string]any and []any values. The API never sends null;
// a key with no value is left out. Ignore keys you don't recognize, since more may be added.
func (pe *PineconeError) Details() map[string]any {
	if pe == nil || pe.details == nil {
		return nil
	}
	return deepCopyJSONObject(*pe.details)
}

func deepCopyJSONObject(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCopyJSONValue(v)
	}
	return out
}

func deepCopyJSONValue(v any) any {
	switch v := v.(type) {
	case map[string]any:
		return deepCopyJSONObject(v)
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = deepCopyJSONValue(e)
		}
		return out
	default:
		return v
	}
}

// Error returns the formatted Msg.
func (pe *PineconeError) Error() string {
	return fmt.Sprintf("%+v", pe.Msg)
}
