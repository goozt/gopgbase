package ca

import (
	"net/http"

	"github.com/goozt/gopgbase/infra/ca/internal/ca/nodes"
	"github.com/goozt/gopgbase/infra/ca/internal/utils"
)

type CaHandler struct{}

func NewCaHandler() *CaHandler {
	return &CaHandler{}
}

func (h *CaHandler) RegisterRoutes() *http.ServeMux {
	nodesHandler := nodes.NewNodesHandler()
	nodesRouter := http.StripPrefix("/nodes", nodesHandler.RegisterRoutes())

	r := http.NewServeMux()
	r.Handle("GET /nodes/", nodesRouter)
	r.Handle("POST /nodes/", nodesRouter)
	r.HandleFunc("GET /", handleListCaCerts)
	r.HandleFunc("GET /ca.crt", handleGetCaCert)
	r.HandleFunc("POST /client", handleCreateClientCert)
	r.HandleFunc("DELETE /client", handleDeleteClientCert)
	r.HandleFunc("/", utils.HandleNotFound)
	return r
}
