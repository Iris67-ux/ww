package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

// Configuration loaded from environment variables
type Config struct {
	ShifuIP         string
	ShifuPort       string
	ShifuAPIBase    string
	ServerHost      string
	ServerPort      string
	CameraSnapshot  string
}

func loadConfig() *Config {
	return &Config{
		ShifuIP:        getEnv("SHIFU_IP", "127.0.0.1"),
		ShifuPort:      getEnv("SHIFU_PORT", "8080"),
		ShifuAPIBase:   getEnv("SHIFU_API_BASE", ""),
		ServerHost:     getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort:     getEnv("SERVER_PORT", "8081"),
		CameraSnapshot: getEnv("CAMERA_SNAPSHOT_PATH", "/api/v1/camera/snapshot"),
	}
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

// Helper to build the Shifu device API URL
func (c *Config) deviceURL(path string) string {
	base := c.ShifuAPIBase
	if base == "" {
		base = fmt.Sprintf("http://%s:%s", c.ShifuIP, c.ShifuPort)
	}
	return fmt.Sprintf("%s%s", base, path)
}

// Handler for /status
func statusHandler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := cfg.deviceURL("/api/v1/status")
		resp, err := http.Get(target)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to fetch status: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

// Handler for /metrics
func metricsHandler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := cfg.deviceURL("/api/v1/metrics")
		resp, err := http.Get(target)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to fetch metrics: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

// Handler for /upgrade
func upgradeHandler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := cfg.deviceURL("/api/v1/upgrade")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		req, err := http.NewRequest(http.MethodPost, target, bytes.NewBuffer(body))
		if err != nil {
			http.Error(w, "Failed to build request", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to upgrade: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

// Handler for /control
func controlHandler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := cfg.deviceURL("/api/v1/control")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		req, err := http.NewRequest(http.MethodPost, target, bytes.NewBuffer(body))
		if err != nil {
			http.Error(w, "Failed to build request", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to send control command: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

// Handler for /infer
func inferHandler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := cfg.deviceURL("/api/v1/infer")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		req, err := http.NewRequest(http.MethodPost, target, bytes.NewBuffer(body))
		if err != nil {
			http.Error(w, "Failed to build request", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to trigger inference: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

// Handler for /camera
func cameraHandler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// GET a snapshot from the camera and proxy back to HTTP
		target := cfg.deviceURL(cfg.CameraSnapshot)
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		resp, err := client.Get(target)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get camera snapshot: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "image/jpeg"
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func main() {
	cfg := loadConfig()
	mux := http.NewServeMux()
	mux.HandleFunc("/status", statusHandler(cfg))
	mux.HandleFunc("/metrics", metricsHandler(cfg))
	mux.HandleFunc("/upgrade", upgradeHandler(cfg))
	mux.HandleFunc("/control", controlHandler(cfg))
	mux.HandleFunc("/infer", inferHandler(cfg))
	mux.HandleFunc("/camera", cameraHandler(cfg))
	mux.HandleFunc("/healthz", healthzHandler)

	serverAddr := net.JoinHostPort(cfg.ServerHost, cfg.ServerPort)
	server := &http.Server{
		Addr:    serverAddr,
		Handler: mux,
	}

	log.Printf("Shifu driver HTTP server started at %s", serverAddr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}