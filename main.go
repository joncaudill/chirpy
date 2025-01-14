package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
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

func (cfg *apiConfig) resetHits(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Metrics reset\n"))
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func main() {
	fsHits := apiConfig{}
	//set location for files being served
	httpDir := http.Dir(".")
	//create a file server
	fileHandler := http.FileServer(httpDir)
	//create a new serve mux
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("GET /api/healthz", healthzHandler)
	serveMux.HandleFunc("GET /admin/metrics", fsHits.getMetrics)
	serveMux.HandleFunc("POST /admin/reset", fsHits.resetHits)
	//tell the servemux the app url is being handled by the middleware server
	serveMux.Handle("/app/", fsHits.middlewareMetricsInc(http.StripPrefix("/app", fileHandler)))
	server := http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}

}
