package ca

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/goozt/gopgbase/infra/ca/internal/utils"
)

func handleGetCaCert(w http.ResponseWriter, r *http.Request) {
	caCertPath := utils.GetCertDir() + "/ca.crt"
	w.Header().Del("If-Modified-Since")
	w.Header().Del("If-None-Match")
	w.Header().Set("Content-Disposition", "attachment; filename=\"ca.crt\"")
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	http.ServeFile(w, r, caCertPath)
}

func handleListCaCerts(w http.ResponseWriter, r *http.Request) {
	certDir := utils.GetCertDir()
	files, err := os.ReadDir(certDir)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to read cert directory")
		return
	}

	certFiles := []string{}
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".crt" {
			certFiles = append(certFiles, file.Name())
		}
	}

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"certs": certFiles,
	})
}

func handleDeleteClientCert(w http.ResponseWriter, r *http.Request) {
	certDir := utils.GetCertDir()
	certFile := filepath.Join(certDir, "client.root.crt")
	keyFile := filepath.Join(certDir, "client.root.key")

	os.Remove(certFile)
	os.Remove(keyFile)

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message":   "Client certificate deleted successfully",
		"cert-file": certFile,
		"key-file":  keyFile,
	})
}

func handleCreateClientCert(w http.ResponseWriter, r *http.Request) {
	certDir := utils.GetCertDir()
	caKey := filepath.Join(certDir, "ca.key")
	certFile := filepath.Join(certDir, "client.root.crt")
	keyFile := filepath.Join(certDir, "client.root.key")
	_, errCert := os.Stat(certFile)
	_, errKey := os.Stat(keyFile)

	if errCert == nil && errKey == nil {
		utils.WriteError(w, http.StatusConflict, "Client certificate and key already exist")
		return
	}

	os.Remove(certFile)
	os.Remove(keyFile)

	cmd := exec.Command("cockroach", "cert", "create-client", "root",
		"--certs-dir="+certDir,
		"--ca-key="+caKey,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "Failed to generate cert",
			"details": string(output),
		})
		return
	}

	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Command succeeded but certificate file was not found",
		})
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message":   "Client certificate generated successfully",
		"cert-file": certFile,
		"key-file":  keyFile,
	})
}
