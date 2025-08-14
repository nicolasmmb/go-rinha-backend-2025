package domain

type SummaryItem struct {
	TotalRequests int64   `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

type Summary struct {
	Default  SummaryItem `json:"default"`
	Fallback SummaryItem `json:"fallback"`
}
