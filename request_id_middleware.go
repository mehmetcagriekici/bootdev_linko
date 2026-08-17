package main

import (
	"crypto/rand"
	"net/http"
)

func requestIdMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request)  {
	  requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			r.Header.Set("X-Request-ID", rand.Text())
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
		})
}
