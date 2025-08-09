package database_service

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// Simple filters matching the diagram needs without overengineering
type ApplicationFilter struct {
	// Optional
	MinAge        *int
	MaxAge        *int
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	StatusEquals  *Status
}

// FindApplications returns applications filtered by basic fields.
// This keeps the query intentionally straightforward and readable.
func (db *DB) FindApplications(ctx context.Context, f ApplicationFilter, limit int, offset int) ([]Application, error) {
	// Build WHERE clause in a very explicit way
	where := "WHERE 1=1"
	args := []any{}

	if f.StatusEquals != nil {
		args = append(args, *f.StatusEquals)
		where += " AND status = $" + strconv.Itoa(len(args))
	}
	if f.CreatedAfter != nil {
		args = append(args, *f.CreatedAfter)
		where += " AND created_at >= $" + strconv.Itoa(len(args))
	}
	if f.CreatedBefore != nil {
		args = append(args, *f.CreatedBefore)
		where += " AND created_at <= $" + strconv.Itoa(len(args))
	}
	// Age filter requires join with users
	join := ""
	if f.MinAge != nil || f.MaxAge != nil {
		join = " JOIN users u ON u.id = a.user_id"
		if f.MinAge != nil {
			args = append(args, *f.MinAge)
			where += " AND u.age >= $" + strconv.Itoa(len(args))
		}
		if f.MaxAge != nil {
			args = append(args, *f.MaxAge)
			where += " AND u.age <= $" + strconv.Itoa(len(args))
		}
	}

	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	sql := "SELECT a.id, a.user_id, a.answers, a.status, a.created_at, a.updated_at FROM applications a" + join + " " + where + " ORDER BY a.created_at DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := db.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Application
	for rows.Next() {
		var a Application
		var raw []byte
		if err := rows.Scan(&a.ID, &a.UserID, &raw, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &a.Answers); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
