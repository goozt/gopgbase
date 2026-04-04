package nodes

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goozt/gopgbase/infra/ca/internal/utils"
)

func handleListNodesCerts(w http.ResponseWriter, r *http.Request) {
	certDir := utils.GetCertDir()
	files, err := os.ReadDir(certDir)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to read cert directory")
		return
	}

	certFiles := []string{}
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "node-") && filepath.Ext(file.Name()) == ".crt" {
			certFiles = append(certFiles, file.Name())
		}
	}

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"certs": certFiles,
	})
}

func handleCreateNodeCert(w http.ResponseWriter, r *http.Request) {
	certDir := utils.GetCertDir()
	caKey := filepath.Join(certDir, "ca.key")
	hostnames := strings.TrimSpace(r.URL.Query().Get("hostnames"))
	if hostnames == "" {
		utils.WriteError(w, http.StatusBadRequest, "Missing host name")
		return
	}

	hostnameList := strings.Split(hostnames, ",")
	argList := append([]string{"cert", "create-node"}, hostnameList...)
	argList = append(argList, []string{
		"--certs-dir=" + certDir,
		"--ca-key=" + caKey,
	}...)
	cmd := exec.Command("cockroach", argList...)

	if output, err := cmd.CombinedOutput(); err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "Failed to generate cert",
			"details": string(output),
		})
		return
	}

	nodeName := strings.Split(hostnames, ",")[0]
	certFile := filepath.Join(certDir, nodeName+".crt")
	keyFile := filepath.Join(certDir, nodeName+".key")

	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Command succeeded but certificate file was not found",
		})
		return
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		utils.WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Command succeeded but key file was not found",
		})
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message":   "Node certificate generated successfully",
		"node":      nodeName,
		"cert-file": certFile,
		"key-file":  keyFile,
	})
}
