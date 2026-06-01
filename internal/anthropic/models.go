package anthropic

type ModelInfo struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

type ModelList struct {
	Data    []ModelInfo `json:"data"`
	FirstID string      `json:"first_id,omitempty"`
	LastID  string      `json:"last_id,omitempty"`
	HasMore bool        `json:"has_more"`
}
