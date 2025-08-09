package database_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgx pool and exposes minimal internal helpers for the project.
type DB struct {
	pool *pgxpool.Pool
}

// Connect creates a connection pool. Example dsn:
// postgres://user:pass@host:5432/dbname?sslmode=disable
func Connect(ctx context.Context, dsn string, maxConns int32) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	// Simple ping to validate connectivity
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(cctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{pool: pool}, nil
}

func (db *DB) Close() {
	if db != nil && db.pool != nil {
		db.pool.Close()
	}
}

// withActor sets application.actor for audit triggers inside a transaction.
func withActor(ctx context.Context, tx pgx.Tx, actor string) error {
	if strings.TrimSpace(actor) == "" {
		// leave as default NULL to avoid noisy logs
		return nil
	}
	_, err := tx.Exec(ctx, "SELECT set_config('application.actor', $1, true)", actor)
	return err
}

// UpsertUser inserts or updates a user row based on Discord user id.
func (db *DB) UpsertUser(ctx context.Context, actor string, u User) (User, error) {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := withActor(ctx, tx, actor); err != nil {
		return User{}, err
	}

	// Returning full row ensures defaults are populated.
	row := tx.QueryRow(ctx, `
        INSERT INTO users (discord_user_id, discord_username, minecraft_name, age)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (discord_user_id)
        DO UPDATE SET discord_username = EXCLUDED.discord_username,
                      minecraft_name   = COALESCE(EXCLUDED.minecraft_name, users.minecraft_name),
                      age              = COALESCE(EXCLUDED.age, users.age)
        RETURNING id, discord_user_id, discord_username, minecraft_name, age, created_at, updated_at
    `, u.DiscordUserID, u.DiscordUsername, u.MinecraftName, u.Age)

	var out User
	if err := row.Scan(&out.ID, &out.DiscordUserID, &out.DiscordUsername, &out.MinecraftName, &out.Age, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return out, nil
}

// SetMinecraftName updates minecraft_name for a user.
func (db *DB) SetMinecraftName(ctx context.Context, actor string, userID string, minecraftName *string) (User, error) {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := withActor(ctx, tx, actor); err != nil {
		return User{}, err
	}

	row := tx.QueryRow(ctx, `
        UPDATE users SET minecraft_name = $2
        WHERE id = $1
        RETURNING id, discord_user_id, discord_username, minecraft_name, age, created_at, updated_at
    `, userID, minecraftName)

	var out User
	if err := row.Scan(&out.ID, &out.DiscordUserID, &out.DiscordUsername, &out.MinecraftName, &out.Age, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return out, nil
}

// CreateOrUpdateApplication sets or updates an application for a user.
func (db *DB) CreateOrUpdateApplication(ctx context.Context, actor string, app Application) (Application, error) {
	if app.UserID == "" {
		return Application{}, errors.New("user_id required")
	}
	// Ensure answers is non-nil for JSON encoding
	if app.Answers == nil {
		app.Answers = map[string]any{}
	}
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Application{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := withActor(ctx, tx, actor); err != nil {
		return Application{}, err
	}

	row := tx.QueryRow(ctx, `
        INSERT INTO applications (user_id, answers, status)
        VALUES ($1, $2, $3)
        ON CONFLICT (user_id)
        DO UPDATE SET answers = EXCLUDED.answers, status = EXCLUDED.status
        RETURNING id, user_id, answers, status, created_at, updated_at
    `, app.UserID, app.Answers, app.Status)

	var out Application
	var answersRaw []byte
	if err := row.Scan(&out.ID, &out.UserID, &answersRaw, &out.Status, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return Application{}, err
	}
	if err := json.Unmarshal(answersRaw, &out.Answers); err != nil {
		return Application{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Application{}, err
	}
	return out, nil
}

// UpdateApplicationStatus updates just the status.
func (db *DB) UpdateApplicationStatus(ctx context.Context, actor string, applicationID string, status Status) (Application, error) {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Application{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := withActor(ctx, tx, actor); err != nil {
		return Application{}, err
	}

	row := tx.QueryRow(ctx, `
        UPDATE applications SET status = $2
        WHERE id = $1
        RETURNING id, user_id, answers, status, created_at, updated_at
    `, applicationID, status)

	var out Application
	var answersRaw []byte
	if err := row.Scan(&out.ID, &out.UserID, &answersRaw, &out.Status, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return Application{}, err
	}
	if err := json.Unmarshal(answersRaw, &out.Answers); err != nil {
		return Application{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Application{}, err
	}
	return out, nil
}

// GetUserByDiscordID finds a user by discord_user_id.
func (db *DB) GetUserByDiscordID(ctx context.Context, discordUserID int64) (*User, error) {
	row := db.pool.QueryRow(ctx, `
        SELECT id, discord_user_id, discord_username, minecraft_name, age, created_at, updated_at
        FROM users WHERE discord_user_id = $1
    `, discordUserID)
	var u User
	if err := row.Scan(&u.ID, &u.DiscordUserID, &u.DiscordUsername, &u.MinecraftName, &u.Age, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// GetApplicationByUser returns the application for a user if present.
func (db *DB) GetApplicationByUser(ctx context.Context, userID string) (*Application, error) {
	row := db.pool.QueryRow(ctx, `
        SELECT id, user_id, answers, status, created_at, updated_at
        FROM applications WHERE user_id = $1
    `, userID)
	var a Application
	var answersRaw []byte
	if err := row.Scan(&a.ID, &a.UserID, &answersRaw, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(answersRaw, &a.Answers); err != nil {
		return nil, err
	}
	return &a, nil
}

// ListenAppEvents subscribes to the `app_events` channel and emits decoded events.
// Cancel the provided context to stop listening; the returned error channel will then close.
func (db *DB) ListenAppEvents(ctx context.Context) (<-chan AppEvent, <-chan error, error) {
	// Dedicated connection for LISTEN/NOTIFY is recommended
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Use a dedicated *pgx.Conn for LISTEN / NOTIFY
	raw := conn.Conn()
	if _, err := raw.Exec(ctx, "LISTEN app_events"); err != nil {
		conn.Release()
		return nil, nil, err
	}

	events := make(chan AppEvent)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)
		defer func() {
			// try to unlisten, then release
			_, _ = raw.Exec(context.Background(), "UNLISTEN app_events")
			conn.Release()
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Wait for notification with a timeout to allow context checks
			ctxWait, cancel := context.WithTimeout(ctx, 55*time.Second)
			notif, err := raw.WaitForNotification(ctxWait)
			cancel()
			if err != nil {
				// If deadline exceeded due to timeout, keep loop alive to re-check ctx
				if pgconn.Timeout(err) || errors.Is(err, context.DeadlineExceeded) {
					continue
				}
				if errors.Is(err, context.Canceled) {
					return
				}
				errs <- fmt.Errorf("listen error: %w", err)
				return
			}

			var ev AppEvent
			if err := json.Unmarshal([]byte(notif.Payload), &ev); err != nil {
				errs <- fmt.Errorf("decode notify payload: %w", err)
				continue
			}
			events <- ev
		}
	}()

	return events, errs, nil
}
