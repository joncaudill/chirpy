package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/joncaudill/chirpy/internal/database"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	jwt_secret     string
	polka_key      string
}

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
	TokenJWT     string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
}

type AuthUser struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type PolkaHook struct {
	Event string `json:"event"`
	Data  struct {
		UserID uuid.UUID `json:"user_id"`
	} `json:"data"`
}

type chirp struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserId    uuid.UUID `json:"user_id"`
}

type chirpError struct {
	Error string `json:"error"`
}

// middleware to increment the hit counter
// when this middleware is attached to a handler
// it returns a handlerfunc that increments the hit counter
// and then calls the next handler
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func profanityFilter(body string) string {
	//list of profanities
	profanities := []string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Fields(body)
	for idx, word := range words {
		for _, profanity := range profanities {
			if strings.ToLower(word) == profanity {
				words[idx] = "****"
			}
		}
	}
	return strings.Join(words, " ")
}

func errHandler(w http.ResponseWriter, err error, statusParm ...int) {
	status := http.StatusInternalServerError
	if len(statusParm) > 0 {
		status = statusParm[0]
	}
	respBody := chirpError{Error: err.Error()}
	resp, _ := json.Marshal(respBody)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(resp)
}

func main() {
	godotenv.Load()
	pform := os.Getenv("PLATFORM")
	jwtSecret := os.Getenv("JWT_SECRET")
	dbURL := os.Getenv("DB_URL")
	polkaKey := os.Getenv("POLKA_KEY")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(err)
	}
	dbQueries := database.New(db)
	config := apiConfig{db: dbQueries, platform: pform, jwt_secret: jwtSecret, polka_key: polkaKey}
	//set location for files being served
	httpDir := http.Dir(".")
	//create a file server
	fileHandler := http.FileServer(httpDir)
	//create a new serve mux
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("GET /api/healthz", healthzHandler)
	serveMux.HandleFunc("GET /admin/metrics", config.getMetrics)
	serveMux.HandleFunc("GET /api/chirps/", config.chirpsGetHandler)
	serveMux.HandleFunc("POST /api/chirps", config.chirpsPostHandler)
	serveMux.HandleFunc("GET /api/chirps/{chirpID}", config.chirpsGetOneHandler)
	serveMux.HandleFunc("DELETE /api/chirps/{chirpID}", config.chirpsDeleteOneHandler)
	serveMux.HandleFunc("POST /admin/reset", config.reset)
	serveMux.HandleFunc("POST /api/users", config.createUser)
	serveMux.HandleFunc("PUT /api/users", config.updateUser)
	serveMux.HandleFunc("POST /api/login", config.loginUser)
	serveMux.HandleFunc("POST /api/refresh", config.updateJWTToken)
	serveMux.HandleFunc("POST /api/revoke", config.revokeRefreshToken)
	serveMux.HandleFunc("POST /api/polka/webhooks", config.handlePolkaWebhook)
	//tell the servemux the app url is being handled by the middleware server
	serveMux.Handle("/app/", config.middlewareMetricsInc(http.StripPrefix("/app", fileHandler)))
	server := http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}

}
