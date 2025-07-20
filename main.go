package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"fshubhook/fswebhook"
)

// loggingResponseWriter is a wrapper around http.ResponseWriter that allows us to capture the status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware logs the details of each request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := newLoggingResponseWriter(w)
		next.ServeHTTP(lrw, r)
		duration := time.Since(start)
		log.Printf("[%s] %s %d %s", r.Method, r.RequestURI, lrw.statusCode, duration)
	})
}

func main() {
	fswebhook.InitDB()

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/hello", fswebhook.HelloHandler)
	http.HandleFunc("/flights", fswebhook.FlightsHandler)
	http.HandleFunc("/webhook/flight-completed", fswebhook.FlightCompletedHandler)

	// Wrap the default ServeMux with the logging middleware.
	handler := loggingMiddleware(http.DefaultServeMux)

	fmt.Println("Server starting on port 8080...")
	if err := http.ListenAndServe("0.0.0.0:8080", handler); err != nil {
		log.Fatalf("Error starting server: %s", err)
	}
}
