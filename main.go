package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	client, err := newEmailClient()
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/receipt", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var order Order
		if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
			http.Error(w, "invalid order", http.StatusBadRequest)
			return
		}
		out, err := sendReceipt(context.Background(), client, order)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	})
	log.Println("receipt service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
