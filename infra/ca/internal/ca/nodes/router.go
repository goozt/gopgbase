package nodes

import (
	"net/http"

	"github.com/goozt/gopgbase/infra/ca/internal/utils"
)

type NodesHandler struct{}

func NewNodesHandler() *NodesHandler {
	return &NodesHandler{}
}

func (h *NodesHandler) RegisterRoutes() *http.ServeMux {
	r := http.NewServeMux()
	r.HandleFunc("GET /", handleListNodesCerts)
	// r.HandleFunc("GET /ca.crt", handleGetCaCert)
	r.HandleFunc("POST /node", handleCreateNodeCert)
	// r.HandleFunc("DELETE /client", handleDeleteClientCert)
	r.HandleFunc("/", utils.HandleNotFound)
	return r
}
