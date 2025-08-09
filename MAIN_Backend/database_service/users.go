package database_service

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// UpdateUserProfile updates basic user fields required by the application form.
func (db *DB) UpdateUserProfile(ctx context.Context, actor string, userID string, age *int16, minecraftName *string) (User, error) {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := withActor(ctx, tx, actor); err != nil {
		return User{}, err
	}
	row := tx.QueryRow(ctx, `
        UPDATE users SET age = COALESCE($2, age), minecraft_name = $3
        WHERE id = $1
        RETURNING id, discord_user_id, discord_username, minecraft_name, age, created_at, updated_at
    `, userID, age, minecraftName)

	var out User
	if err := row.Scan(&out.ID, &out.DiscordUserID, &out.DiscordUsername, &out.MinecraftName, &out.Age, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return out, nil
}
