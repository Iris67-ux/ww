package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment variable names
const (
	EnvServerHost     = "SERVER_HOST"
	EnvServerPort     = "SERVER_PORT"
	EnvDeviceIP       = "DEVICE_IP"
	EnvMqttHost       = "MQTT_HOST"
	EnvMqttPort       = "MQTT_PORT"
	EnvModbusPort     = "MODBUS_PORT"
	EnvS7Port         = "S7_PORT"
	EnvVideoAPIUrl    = "VIDEO_API_URL"
	EnvVideoAPIKey    = "VIDEO_API_KEY"
	EnvTelemetryAPI   = "TELEMETRY_API"
	EnvStatusAPI      = "STATUS_API"
	EnvControlAPI     = "CONTROL_API"
	EnvOTAApi         = "OTA_API"
)

// Helper: Required environment variable
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required environment variable: %s", key)
	}
	return v
}

// Helper: Optional environment variable with default
func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

// ========== Data Structures ==========

type StatusResponse struct {
	DeviceName   string                 `json:"device_name"`
	Model        string                 `json:"model"`
	Manufacturer string                 `json:"manufacturer"`
	Status       string                 `json:"status"`
	Timestamp    int64                  `json:"timestamp"`
	Metrics      map[string]interface{} `json:"metrics,omitempty"`
}

type TelemetryResponse struct {
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

type OTARequest struct {
	FirmwareURL string `json:"firmware_url"`
	Version     string `json:"version"`
}

type ControlRequest struct {
	Command string                 `json:"command"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// ========== Video Stream Proxy ==========

func streamVideo(w http.ResponseWriter, r *http.Request) {
	videoAPI := mustEnv(EnvVideoAPIUrl)
	apiKey := os.Getenv(EnvVideoAPIKey)

	// For demonstration, we expect the video API to respond with an MJPEG stream
	client := &http.Client{
		Timeout: 0, // no timeout for streaming
	}
	req, err := http.NewRequest("GET", videoAPI, nil)
	if err != nil {
		http.Error(w, "Error preparing video stream request", http.StatusInternalServerError)
		return
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Unable to reach video source", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Video source returned: "+resp.Status, http.StatusBadGateway)
		return
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/") && !strings.HasPrefix(ct, "video/") {
		http.Error(w, "Upstream video is not a compatible stream", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	io.Copy(w, resp.Body)
}

// ========== Telemetry Proxy ==========

func fetchTelemetry(w http.ResponseWriter, r *http.Request) {
	telemetryAPI := mustEnv(EnvTelemetryAPI)
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", telemetryAPI, nil)
	if err != nil {
		http.Error(w, "Failed to prepare telemetry request", http.StatusInternalServerError)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to fetch telemetry data", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// ========== Status Proxy ==========

func fetchStatus(w http.ResponseWriter, r *http.Request) {
	statusAPI := mustEnv(EnvStatusAPI)
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", statusAPI, nil)
	if err != nil {
		http.Error(w, "Failed to prepare status request", http.StatusInternalServerError)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to fetch status data", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// ========== OTA Upgrade Proxy ==========

func handleOTA(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	otaAPI := mustEnv(EnvOTAApi)
	var otaReq OTARequest
	if err := json.NewDecoder(r.Body).Decode(&otaReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	payload, _ := json.Marshal(otaReq)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", otaAPI, bytes.NewReader(payload))
	if err != nil {
		http.Error(w, "Failed to prepare OTA request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "OTA upgrade failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// ========== Device Control Proxy ==========

func handleControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	controlAPI := mustEnv(EnvControlAPI)
	var ctrlReq ControlRequest
	if err := json.NewDecoder(r.Body).Decode(&ctrlReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	payload, _ := json.Marshal(ctrlReq)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", controlAPI, bytes.NewReader(payload))
	if err != nil {
		http.Error(w, "Failed to prepare control command", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to send control command", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// ========== Main and Routing ==========

func main() {
	// Required env vars
	host := getEnv(EnvServerHost, "0.0.0.0")
	port := getEnv(EnvServerPort, "8080")

	// Non-protocol ports are not used here, but can be enforced if needed
	// (MQTT, Modbus, S7, etc.) - not implemented as HTTP endpoints

	http.HandleFunc("/status", fetchStatus)
	http.HandleFunc("/telemetry", fetchTelemetry)
	http.HandleFunc("/video", streamVideo)
	http.HandleFunc("/ota", handleOTA)
	http.HandleFunc("/control", handleControl)

	addr := host + ":" + port
	log.Printf("Shifu PAIOS HTTP Driver starting at %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}