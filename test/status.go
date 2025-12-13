package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func upHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "UP")
}

// simulate slow response (>1000ms)
func degradedHandler(w http.ResponseWriter, req *http.Request) {
	time.Sleep(1200 * time.Millisecond)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "DEGRADED")
}

func downHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintln(w, "DOWN")
}

func main() {
	http.HandleFunc("/up", upHandler)
	http.HandleFunc("/degraded", degradedHandler)
	http.HandleFunc("/down", downHandler)

	log.Println("test server starting on :9999")

	if err := http.ListenAndServe(":9999", nil); err != nil {
		log.Fatalf("test server failed: %v", err)
	}
}
