package middleware

import (
	"net/http"

	"arthik/config"
)

// SecurityHeaders adds security headers to all responses
func SecurityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Apply all security headers
		for key, value := range config.SecurityHeaders {
			w.Header().Set(key, value)
		}

		// Only set HSTS if using HTTPS
		// w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		next(w, r)
	}
}

// CORS middleware for handling cross-origin requests (if needed)
func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Uncomment and configure if you need CORS
		// w.Header().Set("Access-Control-Allow-Origin", "*")
		// w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		// w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		// if r.Method == "OPTIONS" {
		//     w.WriteHeader(http.StatusOK)
		//     return
		// }
		
		next(w, r)
	}
}

// RequestLogger logs incoming requests
func RequestLogger(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// You can add logging here if needed
		// log.Printf("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next(w, r)
	}
}