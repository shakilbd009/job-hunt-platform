package model

import "testing"

func TestCreateRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateRequest
		wantErr string
	}{
		{
			name: "happy path",
			req:  CreateRequest{Company: "Acme", Role: "Engineer"},
		},
		{
			name:    "missing company",
			req:     CreateRequest{Role: "Engineer"},
			wantErr: "company is required",
		},
		{
			name:    "missing role",
			req:     CreateRequest{Company: "Acme"},
			wantErr: "role is required",
		},
		{
			name:    "invalid status",
			req:     CreateRequest{Company: "Acme", Role: "Engineer", Status: "bogus"},
			wantErr: `invalid status "bogus"`,
		},
		{
			name: "valid status",
			req:  CreateRequest{Company: "Acme", Role: "Engineer", Status: "applied"},
		},
		{
			name: "salary_min greater than salary_max",
			req: CreateRequest{
				Company:   "Acme",
				Role:      "Engineer",
				SalaryMin: intPtr(200000),
				SalaryMax: intPtr(100000),
			},
			wantErr: "salary_min cannot be greater than salary_max",
		},
		{
			name: "salary_min equals salary_max allowed",
			req: CreateRequest{
				Company:   "Acme",
				Role:      "Engineer",
				SalaryMin: intPtr(150000),
				SalaryMax: intPtr(150000),
			},
		},
		{
			name: "salary_min only no error",
			req: CreateRequest{
				Company:   "Acme",
				Role:      "Engineer",
				SalaryMin: intPtr(100000),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr && !contains(got, tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, got)
			}
		})
	}
}

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		wantErr bool
	}{
		{name: "valid status", status: "applied", wantErr: false},
		{name: "invalid status", status: "nonexistent", wantErr: true},
		{name: "empty string allowed", status: "", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatus(tt.status)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func intPtr(v int) *int { return &v }

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
