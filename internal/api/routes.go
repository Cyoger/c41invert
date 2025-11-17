package api

import (
	"log"
	"net/http"
)

func SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", HealthHandler)
	mux.HandleFunc("/convert", ConvertHandler)

	// Catch-all 404 handler
	mux.HandleFunc("/", NotFoundHandler)

	return corsMiddleware(mux)
}

func corsMiddleware(handler *http.ServeMux) *http.ServeMux {
	wrappedMux := http.NewServeMux()

	wrappedMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)

		handler.ServeHTTP(w, r)
	})

	return wrappedMux
}