package main

import (
	"net/http"
	"time"

	"github.com/goozt/gopgbase/infra/ca/internal/utils"
)

func handleHealth(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}
