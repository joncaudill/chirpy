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
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Hits: %d\n", numHits)))
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
	serveMux.HandleFunc("/healthz", healthzHandler)
	serveMux.HandleFunc("/metrics", fsHits.getMetrics)
	serveMux.HandleFunc("/reset", fsHits.resetHits)
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
