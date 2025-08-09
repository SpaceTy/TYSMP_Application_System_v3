package database_service

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ExchangeToken verifies a token, revokes it, and creates a new token for the same user.
// Returns the user and the newly created token.
func (db *DB) ExchangeToken(ctx context.Context, actor string, token string) (User, LoginToken, error) {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, LoginToken{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := withActor(ctx, tx, actor); err != nil {
		return User{}, LoginToken{}, err
	}

	// lock the token row to avoid double-spend
	var userID string
	err = tx.QueryRow(ctx, `
        SELECT user_id FROM login_tokens
        WHERE token = $1 AND revoked = false AND expires_at > now()
        FOR UPDATE
    `, token).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, LoginToken{}, ErrInvalidOrExpiredToken
		}
		return User{}, LoginToken{}, err
	}

	if _, err := tx.Exec(ctx, `UPDATE login_tokens SET revoked = true WHERE token = $1`, token); err != nil {
		return User{}, LoginToken{}, err
	}

	// create replacement token
	var newTok LoginToken
	if err := tx.QueryRow(ctx, `
        INSERT INTO login_tokens (user_id, token, added_at, expires_at)
        VALUES ($1, uuid_generate_v4(), now(), now() + interval '15 minutes')
        RETURNING id, user_id, token, added_at, expires_at, revoked
    `, userID).Scan(&newTok.ID, &newTok.UserID, &newTok.Token, &newTok.AddedAt, &newTok.ExpiresAt, &newTok.Revoked); err != nil {
		return User{}, LoginToken{}, err
	}

	// fetch user details
	var u User
	if err := tx.QueryRow(ctx, `
        SELECT id, discord_user_id, discord_username, minecraft_name, age, created_at, updated_at
        FROM users WHERE id = $1
    `, userID).Scan(&u.ID, &u.DiscordUserID, &u.DiscordUsername, &u.MinecraftName, &u.Age, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return User{}, LoginToken{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, LoginToken{}, err
	}
	return u, newTok, nil
}

var ErrInvalidOrExpiredToken = errors.New("invalid or expired token")

// ConsumeToken marks a token as revoked after validating it's still active and returns the associated user.
func (db *DB) ConsumeToken(ctx context.Context, actor string, token string) (User, error) {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := withActor(ctx, tx, actor); err != nil {
		return User{}, err
	}

	var userID string
	err = tx.QueryRow(ctx, `
        SELECT user_id FROM login_tokens
        WHERE token = $1 AND revoked = false AND expires_at > now()
        FOR UPDATE
    `, token).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrInvalidOrExpiredToken
		}
		return User{}, err
	}

	if _, err := tx.Exec(ctx, `UPDATE login_tokens SET revoked = true WHERE token = $1`, token); err != nil {
		return User{}, err
	}

	var u User
	if err := tx.QueryRow(ctx, `
        SELECT id, discord_user_id, discord_username, minecraft_name, age, created_at, updated_at
        FROM users WHERE id = $1
    `, userID).Scan(&u.ID, &u.DiscordUserID, &u.DiscordUsername, &u.MinecraftName, &u.Age, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return u, nil
}

// CreateOrRotateLoginToken ensures a user exists/updated and creates a fresh 15m token.
// This function is intended to be called by the discord bot (or any orchestrator)
// which already knows the Discord snowflake and username.
func (db *DB) CreateOrRotateLoginToken(ctx context.Context, actor string, discordUserID int64, discordUsername string) (User, LoginToken, error) {
	// Upsert user first
	user, err := db.UpsertUser(ctx, actor, User{DiscordUserID: discordUserID, DiscordUsername: discordUsername})
	if err != nil {
		return User{}, LoginToken{}, err
	}

	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, LoginToken{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := withActor(ctx, tx, actor); err != nil {
		return User{}, LoginToken{}, err
	}

	if _, err := tx.Exec(ctx, `UPDATE login_tokens SET revoked = true WHERE user_id = $1 AND revoked = false AND expires_at > now()`, user.ID); err != nil {
		return User{}, LoginToken{}, err
	}

	var tok LoginToken
	if err := tx.QueryRow(ctx, `
        INSERT INTO login_tokens (user_id, token, added_at, expires_at)
        VALUES ($1, uuid_generate_v4(), now(), now() + interval '15 minutes')
        RETURNING id, user_id, token, added_at, expires_at, revoked
    `, user.ID).Scan(&tok.ID, &tok.UserID, &tok.Token, &tok.AddedAt, &tok.ExpiresAt, &tok.Revoked); err != nil {
		return User{}, LoginToken{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, LoginToken{}, err
	}
	_ = time.Minute // placeholder to discourage dead-code removal if compiled differently
	return user, tok, nil
}
