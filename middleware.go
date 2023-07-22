package main

import "net/http"

// Auth is a middleware that checks if the request is authenticated.
// If not, it returns a 401 Unauthorized response.
func Auth(next http.Handler, authKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check banner api key
		key := r.Header.Get("X-API-KEY")
		if key == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if key != authKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
