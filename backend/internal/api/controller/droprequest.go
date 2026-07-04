package controller

// dropRequest is the body of a POST /files call.
type dropRequest struct {
	Name        string            `json:"name"`
	ContentType string            `json:"contentType"`
	Size        int64             `json:"size"`
	Meta        map[string]string `json:"meta"`
}
