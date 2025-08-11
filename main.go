package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"fshubhook/fswebhook"

	"github.com/gorilla/handlers"
	"golang.org/x/crypto/acme/autocert"
)

func groupFlightsHandler(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.UserAgent(), "Mobi") {
		http.ServeFile(w, r, "static/group-flights-mobile.html")
		return
	}
	http.ServeFile(w, r, "static/group-flights-desktop.html")
}

func main() {
	// Define a command-line flag to enable the webhook.
	webhookEnabled := flag.Bool("webhook", false, "Enable the flight completed webhook")
	hostname := flag.String("hostname", "", "Hostname for TLS certificate")
	flag.Parse()

	fswebhook.InitDB()

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/group-flights.html", groupFlightsHandler)
	http.HandleFunc("/flights", fswebhook.FlightsHandler)
	http.HandleFunc("/group-flight", fswebhook.GroupFlightHandler)

	// Only register the webhook handler if the flag is set.
	if *webhookEnabled {
		http.HandleFunc("/webhook/flight-completed", fswebhook.FlightCompletedHandler)
		fmt.Println("Flight completed webhook is enabled.")
	}

	// Wrap the default ServeMux with the logging middleware.
	loggedRouter := handlers.LoggingHandler(os.Stdout, http.DefaultServeMux)

	if *hostname != "" {
		certManager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(*hostname),
			Cache:      autocert.DirCache("/etc/certs"),
		}

		server := &http.Server{
			Addr:    ":443",
			Handler: loggedRouter,
			TLSConfig: &tls.Config{
				GetCertificate: certManager.GetCertificate,
				MinVersion:     tls.VersionTLS11,
			},
		}

		go func() {
			fmt.Println("Server starting on port 443 for https...")
			if err := server.ListenAndServeTLS("", ""); err != nil {
				log.Fatalf("Error starting HTTPS server: %s", err)
			}
		}()

		// Serve HTTP, which will handle ACME challenges and serve content.
		fmt.Println("Server starting on port 80 for http...")
		// The HTTPHandler wraps the main router. It will handle ACME challenges
		// and pass other requests to the loggedRouter.
		log.Fatal(http.ListenAndServe(":80", certManager.HTTPHandler(loggedRouter)))

	} else {
		fmt.Println("Server starting on port 8080...")
		if err := http.ListenAndServe("0.0.0.0:8080", loggedRouter); err != nil {
			log.Fatalf("Error starting server: %s", err)
		}
	}
}