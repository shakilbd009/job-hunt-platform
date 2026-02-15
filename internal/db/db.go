package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/shakilbd009/job-hunt-platform/internal/model"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating data directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", dbPath))
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS applications (
		id         TEXT PRIMARY KEY,
		company    TEXT NOT NULL,
		role       TEXT NOT NULL,
		url        TEXT DEFAULT '',
		salary_min INTEGER DEFAULT 0,
		salary_max INTEGER DEFAULT 0,
		location   TEXT DEFAULT '',
		status     TEXT NOT NULL DEFAULT 'wishlist',
		notes      TEXT DEFAULT '',
		applied_at TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	return err
}

func generateID() string {
	return uuid.New().String()[:8]
}

func (s *Store) Count(status string) (int, error) {
	query := "SELECT COUNT(*) FROM applications"
	var args []interface{}

	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}

	var count int
	err := s.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) List(status string, limit, offset int) ([]model.Application, error) {
	query := "SELECT id, company, role, url, salary_min, salary_max, location, status, notes, applied_at, created_at, updated_at FROM applications"
	var args []interface{}

	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []model.Application
	for rows.Next() {
		var a model.Application
		if err := rows.Scan(&a.ID, &a.Company, &a.Role, &a.URL, &a.SalaryMin, &a.SalaryMax, &a.Location, &a.Status, &a.Notes, &a.AppliedAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}
	if apps == nil {
		apps = []model.Application{}
	}
	return apps, rows.Err()
}

func (s *Store) Get(id string) (*model.Application, error) {
	var a model.Application
	err := s.db.QueryRow(
		"SELECT id, company, role, url, salary_min, salary_max, location, status, notes, applied_at, created_at, updated_at FROM applications WHERE id = ?",
		id,
	).Scan(&a.ID, &a.Company, &a.Role, &a.URL, &a.SalaryMin, &a.SalaryMax, &a.Location, &a.Status, &a.Notes, &a.AppliedAt, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) Create(req model.CreateRequest) (*model.Application, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	id := generateID()

	status := req.Status
	if status == "" {
		status = "wishlist"
	}

	salaryMin := 0
	if req.SalaryMin != nil {
		salaryMin = *req.SalaryMin
	}
	salaryMax := 0
	if req.SalaryMax != nil {
		salaryMax = *req.SalaryMax
	}

	_, err := s.db.Exec(
		"INSERT INTO applications (id, company, role, url, salary_min, salary_max, location, status, notes, applied_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, req.Company, req.Role, req.URL, salaryMin, salaryMax, req.Location, status, req.Notes, req.AppliedAt, now, now,
	)
	if err != nil {
		return nil, err
	}

	return s.Get(id)
}

func (s *Store) Update(id string, fields map[string]interface{}) (*model.Application, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	var existing model.Application
	err = tx.QueryRow(
		"SELECT id, company, role, url, salary_min, salary_max, location, status, notes, applied_at, created_at, updated_at FROM applications WHERE id = ?",
		id,
	).Scan(&existing.ID, &existing.Company, &existing.Role, &existing.URL, &existing.SalaryMin, &existing.SalaryMax, &existing.Location, &existing.Status, &existing.Notes, &existing.AppliedAt, &existing.CreatedAt, &existing.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	allowed := map[string]string{
		"company":    "company",
		"role":       "role",
		"url":        "url",
		"salary_min": "salary_min",
		"salary_max": "salary_max",
		"location":   "location",
		"status":     "status",
		"notes":      "notes",
		"applied_at": "applied_at",
	}

	var setClauses []string
	var args []interface{}

	for jsonKey, col := range allowed {
		if val, ok := fields[jsonKey]; ok {
			setClauses = append(setClauses, col+" = ?")
			args = append(args, val)
		}
	}

	if len(setClauses) == 0 {
		return &existing, nil
	}

	setClauses = append(setClauses, "updated_at = ?")
	now := time.Now().UTC().Format(time.RFC3339)
	args = append(args, now)
	args = append(args, id)

	query := fmt.Sprintf("UPDATE applications SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	_, err = tx.Exec(query, args...)
	if err != nil {
		return nil, err
	}

	var updated model.Application
	err = tx.QueryRow(
		"SELECT id, company, role, url, salary_min, salary_max, location, status, notes, applied_at, created_at, updated_at FROM applications WHERE id = ?",
		id,
	).Scan(&updated.ID, &updated.Company, &updated.Role, &updated.URL, &updated.SalaryMin, &updated.SalaryMax, &updated.Location, &updated.Status, &updated.Notes, &updated.AppliedAt, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &updated, nil
}

func (s *Store) Delete(id string) (bool, error) {
	res, err := s.db.Exec("DELETE FROM applications WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
