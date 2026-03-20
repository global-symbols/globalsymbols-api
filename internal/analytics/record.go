package analytics

import "time"

type Record struct {
	CreatedAt         time.Time           `json:"created_at"`
	UserEmail         string              `json:"user_email,omitempty"`
	APIKeyID          string              `json:"api_key_id,omitempty"`
	UserID            string              `json:"user_id,omitempty"`
	IPAddress         string              `json:"ip_address,omitempty"`
	UserAgent         string              `json:"user_agent,omitempty"`
	Method            string              `json:"method"`
	Path              string              `json:"path"`
	QueryParams       map[string][]string `json:"query_params,omitempty"`
	StatusCode        int                 `json:"status_code"`
	ResponseTimeMS    int64               `json:"response_time_ms,omitempty"`
	IsRateLimitBreach bool                `json:"is_rate_limit_breach,omitempty"`
	RemainingQuota    *int                `json:"remaining_quota,omitempty"`
	ErrorMessage      string              `json:"error_message,omitempty"`
}
