package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"fshubhook/fswebhook"

	"github.com/gorilla/handlers"
)

func main() {
	// Define a command-line flag to enable the webhook.
	webhookEnabled := flag.Bool("webhook", false, "Enable the flight completed webhook")
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

	fmt.Println("Server starting on port 8080...")
	if err := http.ListenAndServe("0.0.0.0:8080", loggedRouter); err != nil {
		log.Fatalf("Error starting server: %s", err)
	}
}
