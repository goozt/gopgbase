package utils

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

type caPathStore struct {
	Path  string
	Ready bool
}

var certDir = caPathStore{Path: "", Ready: false}

type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// func RedirectHandler(targetpath string) func(w http.ResponseWriter, r *http.Request) {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Println(targetpath)
// 		http.Redirect(w, r, targetpath, http.StatusTemporaryRedirect)
// 	}
// }

func HandleNotFound(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotFound, "endpoint not found")
}

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, APIError{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
	})
}

func GetCertDir(posPathArgs ...string) string {
	if certDir.Ready {
		return certDir.Path
	}

	if len(posPathArgs) > 0 && posPathArgs[0] != "" {
		path := posPathArgs[0]
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}

	if envDir := os.Getenv("CERTS_DIR"); envDir != "" {
		if info, err := os.Stat(envDir); err == nil && info.IsDir() {
			return envDir
		}
	}

	const defaultPath = "/cockroach/cockroach-certs"
	if info, err := os.Stat(defaultPath); err == nil {
		if !info.IsDir() {
			slog.Error("Default path is not a directory", "path", defaultPath)
			os.Exit(1)
		}
	} else if !os.IsNotExist(err) {
		slog.Error("Failed to access default path", "error", err)
		os.Exit(1)
	}

	return defaultPath
}

func VerifyCertDir(dir string) {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		slog.Error("invalid path", "dir", dir, "error", err)
		os.Exit(1)
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Error("certs directory does not exist", "dir", absPath)
		} else {
			slog.Error("failed to access certs directory", "dir", absPath, "error", err)
		}
		os.Exit(1)
	}
	if !info.IsDir() {
		slog.Error("path is not a directory", "dir", absPath)
		os.Exit(1)
	}
	f, err := os.Open(absPath)
	if err != nil {
		slog.Error("certs directory is not readable (check permissions)", "dir", certDir, "error", err)
		os.Exit(1)
	}
	f.Close()

	certDir.Path = absPath
	certDir.Ready = true
}
