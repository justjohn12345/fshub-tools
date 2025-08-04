package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"fshubhook/fswebhook"

	"github.com/gorilla/handlers"
	"golang.org/x/crypto/acme/autocert"
)

func main() {
	// Define a command-line flag to enable the webhook.
	webhookEnabled := flag.Bool("webhook", true, "Enable the flight completed webhook")
	hostname := flag.String("hostname", "", "Hostname for TLS certificate")
	flag.Parse()

	fswebhook.InitDB()

	http.Handle("/", http.FileServer(http.Dir("./static")))
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
			},
		}

		go func() {
			// Serve HTTP, which will redirect to HTTPS
			h := certManager.HTTPHandler(nil)
			log.Fatal(http.ListenAndServe(":80", h))
		}()

		fmt.Println("Server starting on port 443 for https...")
		if err := server.ListenAndServeTLS("", ""); err != nil {
			log.Fatalf("Error starting server: %s", err)
		}
	} else {
		fmt.Println("Server starting on port 80...")
		if err := http.ListenAndServe("0.0.0.0:80", loggedRouter); err != nil {
			log.Fatalf("Error starting server: %s", err)
		}
	}
}
