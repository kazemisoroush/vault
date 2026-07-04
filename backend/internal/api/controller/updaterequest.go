package controller

// updateRequest is the body of a PATCH /files/{id} call.
type updateRequest struct {
	Name *string            `json:"name"`
	Meta *map[string]string `json:"meta"`
}
