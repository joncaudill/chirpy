package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/joncaudill/chirpy/internal/auth"
	"github.com/joncaudill/chirpy/internal/database"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AuthUser struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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

func (cfg *apiConfig) getMetrics(w http.ResponseWriter, r *http.Request) {
	numHits := cfg.fileserverHits.Load()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	templateString := `
	<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
	</html>
	`
	w.Write([]byte(fmt.Sprintf(templateString, numHits)))
}

func (cfg *apiConfig) reset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
	}
	cfg.fileserverHits.Store(0)
	cfg.db.ResetUsers(context.Background())
	cfg.db.ResetChirps(context.Background())
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Metrics reset\n"))
	w.Write([]byte("Users reset\n"))
	w.Write([]byte("Chirps reset\n"))

}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func (cfg *apiConfig) chirpsPostHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	parameter := chirp{}
	err := decoder.Decode(&parameter)
	if err != nil {
		errHandler(w, fmt.Errorf("error parsing chirp info: %v", err), http.StatusBadRequest)
		return
	}
	if len(parameter.Body) > 140 {
		errHandler(w, fmt.Errorf("chirp is too long"), http.StatusBadRequest)
		return
	}
	cleaned := profanityFilter(parameter.Body)
	newid := uuid.New()
	timeNow := time.Now()

	respBody, _ := cfg.db.CreateChirp(context.Background(), database.CreateChirpParams{
		ID:        newid,
		CreatedAt: timeNow,
		UpdatedAt: timeNow,
		Body:      cleaned,
		UserID:    parameter.UserId,
	})
	respChirp := chirp{}
	respChirp.Id = respBody.ID
	respChirp.CreatedAt = respBody.CreatedAt
	respChirp.UpdatedAt = respBody.UpdatedAt
	respChirp.Body = respBody.Body
	respChirp.UserId = respBody.UserID

	resp, _ := json.Marshal(respChirp)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	w.Write(resp)
}

func (cfg *apiConfig) chirpsGetHandler(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.db.GetAllChirps(context.Background())
	if err != nil {
		errHandler(w, fmt.Errorf("error getting chirps: %v", err))
		return
	}

	chirpsResp := []string{}
	for _, chrp := range chirps {
		parsedChirp := chirp{}
		parsedChirp.Id = chrp.ID
		parsedChirp.CreatedAt = chrp.CreatedAt
		parsedChirp.UpdatedAt = chrp.UpdatedAt
		parsedChirp.Body = chrp.Body
		parsedChirp.UserId = chrp.UserID
		jsonChirp, _ := json.Marshal(parsedChirp)
		chirpsResp = append(chirpsResp, string(jsonChirp))
	}
	jsonResp, err := json.Marshal(chirpsResp)
	if err != nil {
		errHandler(w, fmt.Errorf("error parsing chirps: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResp)

}

func (cfg *apiConfig) chirpsGetOneHandler(w http.ResponseWriter, r *http.Request) {
	chirpID := r.PathValue("chirpID")
	chirpUUID, _ := uuid.Parse(chirpID)
	fmt.Printf("chirpid: %s", chirpID)
	chirpData, err := cfg.db.GetChirpById(context.Background(), chirpUUID)
	if err != nil {
		errHandler(w, fmt.Errorf("error getting chirp: %v", err))
		return
	}
	if chirpData.ID == uuid.Nil {
		errHandler(w, fmt.Errorf("chirp not found"), http.StatusNotFound)
		return
	}
	respChirp := chirp{}
	respChirp.Id = chirpData.ID
	respChirp.CreatedAt = chirpData.CreatedAt
	respChirp.UpdatedAt = chirpData.UpdatedAt
	respChirp.Body = chirpData.Body
	respChirp.UserId = chirpData.UserID

	jsonResp, err := json.Marshal(respChirp)
	if err != nil {
		errHandler(w, fmt.Errorf("error parsing chirp: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResp)
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

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	partUser := AuthUser{}
	parameter := User{}
	err := decoder.Decode(&partUser)
	if err != nil {
		errHandler(w, fmt.Errorf("error parsing user info: %v", err))
		return
	}
	hashedPassword, err := auth.HashPassword(partUser.Password)
	if err != nil {
		errHandler(w, fmt.Errorf("unable to hash password: %v", err))
		return
	}
	ctx := context.Background()
	newUser, err := cfg.db.CreateUser(ctx,
		database.CreateUserParams{
			Email:          partUser.Email,
			HashedPassword: hashedPassword,
		})
	if err != nil {
		errHandler(w, fmt.Errorf("error creating user: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	parameter.ID = newUser.ID
	parameter.CreatedAt = newUser.CreatedAt
	parameter.UpdatedAt = newUser.UpdatedAt
	parameter.Email = newUser.Email
	resp, _ := json.Marshal(parameter)
	w.Write(resp)
}

func (cfg *apiConfig) loginUser(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	partUser := AuthUser{}
	parameter := User{}
	err := decoder.Decode(&partUser)
	if err != nil {
		errHandler(w, fmt.Errorf("error parsing login info: %v", err))
		return
	}
	ctx := context.Background()
	user, err := cfg.db.GetUserByEmail(ctx, partUser.Email)
	if err != nil {
		errHandler(w, fmt.Errorf("error getting user: %v", err))
		return
	}
	if user.ID == uuid.Nil {
		errHandler(w, fmt.Errorf("user not found"), http.StatusNotFound)
		return
	}
	validated := auth.CheckPasswordHash(partUser.Password, user.HashedPassword)
	if !validated {
		errHandler(w, fmt.Errorf("incorrect email or password"), http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	parameter.ID = user.ID
	parameter.CreatedAt = user.CreatedAt
	parameter.UpdatedAt = user.UpdatedAt
	parameter.Email = user.Email
	resp, _ := json.Marshal(parameter)
	w.Write(resp)

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
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(err)
	}
	dbQueries := database.New(db)
	config := apiConfig{db: dbQueries, platform: pform}
	//set location for files being served
	httpDir := http.Dir(".")
	//create a file server
	fileHandler := http.FileServer(httpDir)
	//create a new serve mux
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("GET /api/healthz", healthzHandler)
	serveMux.HandleFunc("GET /admin/metrics", config.getMetrics)
	serveMux.HandleFunc("POST /admin/reset", config.reset)
	serveMux.HandleFunc("POST /api/chirps", config.chirpsPostHandler)
	serveMux.HandleFunc("POST /api/users", config.createUser)
	serveMux.HandleFunc("POST /api/login", config.loginUser)
	serveMux.HandleFunc("GET /api/chirps/", config.chirpsGetHandler)
	serveMux.HandleFunc("GET /api/chirps/{chirpID}", config.chirpsGetOneHandler)
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
