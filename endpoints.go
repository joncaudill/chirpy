package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/joncaudill/chirpy/internal/auth"
	"github.com/joncaudill/chirpy/internal/database"
)

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
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
