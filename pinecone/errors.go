package pinecone

import "fmt"

// PineconeError is the error returned when a Pinecone REST API request (control plane, admin,
// inference, or REST data plane) receives a non-success HTTP response. It is returned as a
// *PineconeError; use errors.As to read Code.
//
// Data-plane calls made over gRPC (vector upsert, query, fetch, update, delete, stats, and namespace
// operations) return gRPC status errors instead; inspect them with status.Code(err) from
// google.golang.org/grpc/status. Network and response-decoding failures are returned as plain
// errors.
type PineconeError struct {
	// Code is the HTTP status code of the response.
	Code int
	// Msg holds a JSON-encoded summary of the response: the status code, the raw body, and the
	// error code, message, and details decoded from the body when available.
	Msg error
}

// Error returns the formatted Msg.
func (pe *PineconeError) Error() string {
	return fmt.Sprintf("%+v", pe.Msg)
}
