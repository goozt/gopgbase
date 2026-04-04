package main

import (
	"net/http"

	"github.com/goozt/gopgbase/infra/ca/internal/ca"
	"github.com/goozt/gopgbase/infra/ca/internal/utils"
)

func registerRoutes(router *http.ServeMux) {
	caHandler := ca.NewCaHandler()
	caRouter := caHandler.RegisterRoutes()

	router.Handle("/ca/", http.StripPrefix("/ca", caRouter))
	router.HandleFunc("GET /health", handleHealth)
	router.HandleFunc("/", utils.HandleNotFound)
}
