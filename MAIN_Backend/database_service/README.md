Internal Go package providing DB access functions for the TYSMP app.

No HTTP endpoints here; import as a library from other services.

Key features:
- Connect to Postgres using pgx pool
- Upsert users and applications
- Update application status and minecraft name
- Simple filtering for applications
- Listen to `app_events` from DB triggers for role/whitelist sync

Example usage (pseudo):

```go
ctx := context.Background()
db, _ := database_service.Connect(ctx, os.Getenv("DATABASE_URL"), 4)
defer db.Close()

user, _ := db.UpsertUser(ctx, "discord:1234", database_service.User{DiscordUserID: 1234, DiscordUsername: "foo"})
app, _ := db.CreateOrUpdateApplication(ctx, "moderator:42", database_service.Application{UserID: user.ID, Status: database_service.StatusApplicant})

events, errs, _ := db.ListenAppEvents(ctx)
_ = events; _ = errs
```


