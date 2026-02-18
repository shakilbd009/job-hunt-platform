package model

import "fmt"

var ValidStatuses = map[string]bool{
	"wishlist":     true,
	"applied":      true,
	"phone_screen": true,
	"interview":    true,
	"offer":        true,
	"accepted":     true,
	"rejected":     true,
	"withdrawn":    true,
	"ghosted":      true,
}

type Application struct {
	ID        string `json:"id"`
	Company   string `json:"company"`
	Role      string `json:"role"`
	URL       string `json:"url"`
	SalaryMin int    `json:"salary_min"`
	SalaryMax int    `json:"salary_max"`
	Location  string `json:"location"`
	Status    string `json:"status"`
	Notes     string `json:"notes"`
	AppliedAt string `json:"applied_at"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type CreateRequest struct {
	Company   string `json:"company"`
	Role      string `json:"role"`
	URL       string `json:"url"`
	SalaryMin *int   `json:"salary_min"`
	SalaryMax *int   `json:"salary_max"`
	Location  string `json:"location"`
	Status    string `json:"status"`
	Notes     string `json:"notes"`
	AppliedAt string `json:"applied_at"`
}

func (r CreateRequest) Validate() error {
	if r.Company == "" {
		return fmt.Errorf("company is required")
	}
	if r.Role == "" {
		return fmt.Errorf("role is required")
	}
	if r.Status != "" && !ValidStatuses[r.Status] {
		return fmt.Errorf("invalid status %q, valid values: wishlist, applied, phone_screen, interview, offer, accepted, rejected, withdrawn, ghosted", r.Status)
	}
	if r.SalaryMin != nil && r.SalaryMax != nil && *r.SalaryMin > *r.SalaryMax {
		return fmt.Errorf("salary_min cannot be greater than salary_max")
	}
	return nil
}

type StatsResponse struct {
	ByStatus       map[string]int `json:"by_status"`
	Total          int            `json:"total"`
	SalaryRange    SalaryRange    `json:"salary_range"`
	RecentActivity RecentActivity `json:"recent_activity"`
}

type SalaryRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
	Avg int `json:"avg"`
}

type RecentActivity struct {
	Last7Days  int `json:"last_7_days"`
	Last30Days int `json:"last_30_days"`
}

func ValidateStatus(status string) error {
	if status != "" && !ValidStatuses[status] {
		return fmt.Errorf("invalid status %q, valid values: wishlist, applied, phone_screen, interview, offer, accepted, rejected, withdrawn, ghosted", status)
	}
	return nil
}
