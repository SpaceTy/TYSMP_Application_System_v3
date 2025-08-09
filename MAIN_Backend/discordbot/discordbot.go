package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
)

// GuildUser represents a concise view of a Discord user in a guild with their role IDs.
type GuildUser struct {
	ID            string   `json:"id"`
	Username      string   `json:"username"`
	Discriminator string   `json:"discriminator"`
	Nickname      string   `json:"nickname,omitempty"`
	Roles         []string `json:"roles"`
}

// GuildUsersResponse is a wrapper object that can be extended later.
type GuildUsersResponse struct {
	GuildID string      `json:"guild_id"`
	Users   []GuildUser `json:"users"`
}

// getGuildUsers fetches guild members and returns simplified user objects with roles.
func getGuildUsers(ctx context.Context, s *discordgo.Session, guildID string) ([]GuildUser, error) {
	// Discord limits list members; use pagination.
	var all []GuildUser
	var after string

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		members, err := s.GuildMembers(guildID, after, 1000)
		if err != nil {
			return nil, err
		}
		if len(members) == 0 {
			break
		}
		for _, m := range members {
			u := m.User
			user := GuildUser{
				ID:            u.ID,
				Username:      u.Username,
				Discriminator: u.Discriminator,
				Nickname:      m.Nick,
				Roles:         append([]string{}, m.Roles...),
			}
			all = append(all, user)
			after = u.ID
		}
		if len(members) < 1000 {
			break
		}
	}
	return all, nil
}

func main() {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	guildID := os.Getenv("DISCORD_GUILD_ID")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if token == "" || guildID == "" {
		log.Fatal("❌ DISCORD_BOT_TOKEN and DISCORD_GUILD_ID must be set")
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("❌ failed to create discord session: %v", err)
	}
	defer session.Close()

	if err := session.Open(); err != nil {
		log.Fatalf("❌ failed to open discord session: %v", err)
	}
	log.Println("Discord bot session established ✨")

	// Very small HTTP API: GET /users returns current guild users with roles
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		users, err := getGuildUsers(ctx, session, guildID)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to fetch guild users: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GuildUsersResponse{GuildID: guildID, Users: users})
	})

	srv := &http.Server{Addr: ":" + port, Handler: mux}
	log.Printf("discordbot listening on :%s\n", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ server error: %v", err)
	}
}
