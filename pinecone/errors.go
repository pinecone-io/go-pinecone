package pinecone

import "fmt"

type PineconeError struct {
	Code int
	Msg  error
}

func (pe *PineconeError) Error() string {
	return fmt.Sprintf("%+v", pe.Msg)
}
