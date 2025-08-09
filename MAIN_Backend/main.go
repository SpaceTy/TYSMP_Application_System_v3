package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	ds "tysmp/main_backend/database_service"
)

type exchangeRequest struct {
	Token string `json:"token"`
}
type exchangeResponse struct {
	DiscordID string `json:"discord_id"`
	Username  string `json:"username"`
	NextToken string `json:"next_token"`
	ExpiresAt string `json:"expires_at"`
}

type submitRequest struct {
	Token             string `json:"token"`
	Age               int16  `json:"age"`
	MinecraftUsername string `json:"minecraft_username"`
	FavouriteAboutMC  string `json:"favourite_about_minecraft"`
	Understanding     string `json:"server_understanding"`
}
type submitResponse struct {
	ApplicationID string `json:"application_id"`
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("❌ DATABASE_URL must be set")
	}

	ctx := context.Background()
	db, err := ds.Connect(ctx, dsn, 6)
	if err != nil {
		log.Fatalf("❌ db connect: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()

	// Basic CORS for test frontend
	cors := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// POST /create-login-token -> for bot or testing to initiate token flow
	mux.HandleFunc("/create-login-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			DiscordID string `json:"discord_id"`
			Username  string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.DiscordID == "" || body.Username == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		idInt, err := strconv.ParseInt(body.DiscordID, 10, 64)
		if err != nil {
			http.Error(w, "invalid discord_id", http.StatusBadRequest)
			return
		}
		cctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		_, tok, err := db.CreateOrRotateLoginToken(cctx, "api:create_token", idInt, body.Username)
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"token":      tok.Token,
			"expires_at": tok.ExpiresAt.UTC().Format(time.RFC3339),
		})
	})

	// Serve test frontend for convenience
	mux.Handle("/", http.FileServer(http.Dir("./test_frontend")))

	// POST /exchange-token -> returns user discord info + new single-use token
	mux.HandleFunc("/exchange-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req exchangeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		cctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		user, tok, err := db.ExchangeToken(cctx, "api:exchange", req.Token)
		if err != nil {
			if err == ds.ErrInvalidOrExpiredToken {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exchangeResponse{
			DiscordID: strconv.FormatInt(user.DiscordUserID, 10),
			Username:  user.DiscordUsername,
			NextToken: tok.Token,
			ExpiresAt: tok.ExpiresAt.UTC().Format(time.RFC3339),
		})
	})

	// POST /submit-application -> consumes token and stores application + updates profile
	mux.HandleFunc("/submit-application", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req submitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Basic validation
		if req.Age < 0 || req.Age > 120 || req.MinecraftUsername == "" {
			http.Error(w, "invalid fields", http.StatusBadRequest)
			return
		}

		cctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		user, err := db.ConsumeToken(cctx, "api:submit", req.Token)
		if err != nil {
			if err == ds.ErrInvalidOrExpiredToken {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}

		// Update user profile (age + MC name)
		_, err = db.UpdateUserProfile(cctx, "api:submit", user.ID, &req.Age, &req.MinecraftUsername)
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}

		// Create or update application with answers
		answers := map[string]any{
			"favourite_about_minecraft": req.FavouriteAboutMC,
			"server_understanding":      req.Understanding,
		}
		app, err := db.CreateOrUpdateApplication(cctx, "api:submit", ds.Application{
			UserID:  user.ID,
			Answers: answers,
			Status:  ds.StatusApplicant,
		})
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(submitResponse{ApplicationID: app.ID})
	})

	addr := ":8081"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("api listening on %s", addr)
	srv := &http.Server{Addr: addr, Handler: cors(mux)}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ server error: %v", err)
	}
}
