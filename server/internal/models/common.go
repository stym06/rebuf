package models

// ListResponse wraps paginated list results.
type ListResponse[T any] struct {
	Data       []T    `json:"data"`
	TotalCount int    `json:"total_count"`
	HasMore    bool   `json:"has_more"`
	Cursor     string `json:"cursor,omitempty"`
}

// ErrorResponse is returned on API errors.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}
