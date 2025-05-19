package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

// TelemetryData represents the real-time telemetry, sensor, video, and AI data structure.
type TelemetryData struct {
	Timestamp        time.Time              `json:"timestamp"`
	SensorData       map[string]interface{} `json:"sensor_data,omitempty"`
	VideoStreamMJPEG string                 `json:"video_stream_mjpeg,omitempty"`
	AIResults        map[string]interface{} `json:"ai_results,omitempty"`
	CustomData       map[string]interface{} `json:"custom_data,omitempty"`
}

// OTAUpdateRequest represents the expected OTA update POST payload.
type OTAUpdateRequest struct {
	Version     string                 `json:"version"`
	URL         string                 `json:"url"`
	Checksum    string                 `json:"checksum"`
	ExtraParams map[string]interface{} `json:"extra_params,omitempty"`
}

// ControlCommandRequest represents the expected control POST payload.
type ControlCommandRequest struct {
	Command     string                 `json:"command"`
	Parameters  map[string]interface{} `json:"parameters"`
}

var (
	deviceIP         = os.Getenv("DEVICE_IP")
	serverHost       = os.Getenv("SERVER_HOST")
	serverPort       = os.Getenv("SERVER_PORT")
	telemetryTimeout = getenvInt("TELEMETRY_TIMEOUT", 5) // seconds
	videoMJPEGPort   = os.Getenv("VIDEO_MJPEG_PORT")
)

func getenvInt(env string, def int) int {
	if v := os.Getenv(env); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func main() {
	if serverHost == "" {
		serverHost = "0.0.0.0"
	}
	if serverPort == "" {
		serverPort = "8080"
	}

	http.HandleFunc("/telemetry", getTelemetry)
	http.HandleFunc("/ota", otaHandler)
	http.HandleFunc("/control", controlHandler)
	if videoMJPEGPort != "" {
		http.HandleFunc("/telemetry/video", mjpegProxyHandler)
	}

	addr := net.JoinHostPort(serverHost, serverPort)
	log.Printf("Shifu PAIO driver HTTP server listening at %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// getTelemetry handles GET /telemetry. Returns sensor, AI, and (optionally) video data.
func getTelemetry(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(telemetryTimeout)*time.Second)
	defer cancel()

	telemetry := TelemetryData{
		Timestamp: time.Now(),
	}

	// Example: Fetch telemetry from the device (mocked)
	telemetry.SensorData = fetchSensorData(ctx)
	telemetry.AIResults = fetchAIResults(ctx)
	telemetry.CustomData = fetchCustomDeviceData(ctx)

	// If MJPEG video configured, provide video endpoint link
	if videoMJPEGPort != "" {
		telemetry.VideoStreamMJPEG = getVideoHTTPURL()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(telemetry)
}

// otaHandler handles POST /ota for OTA firmware/software upgrades.
func otaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req OTAUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// Mock OTA update trigger
	go performOTAUpdate(req)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"OTA update initiated"}`))
}

// controlHandler handles POST /control for device commands.
func controlHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ControlCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// Mock device control
	resp := performDeviceControl(req)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// mjpegProxyHandler proxies MJPEG video stream from device to HTTP for browser consumption.
// GET /telemetry/video
func mjpegProxyHandler(w http.ResponseWriter, r *http.Request) {
	if videoMJPEGPort == "" || deviceIP == "" {
		http.Error(w, "Video stream not configured", http.StatusNotFound)
		return
	}
	deviceURL := "http://" + net.JoinHostPort(deviceIP, videoMJPEGPort) + "/"
	proxyReq, err := http.NewRequestWithContext(r.Context(), "GET", deviceURL, nil)
	if err != nil {
		http.Error(w, "Failed to prepare video stream", http.StatusInternalServerError)
		return
	}
	client := http.Client{
		Timeout: 0, // No timeout for streaming
	}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Failed to connect to video stream", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// getVideoHTTPURL returns the HTTP URL for MJPEG stream for browser
func getVideoHTTPURL() string {
	host := serverHost
	if host == "0.0.0.0" || host == "" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, serverPort) + "/telemetry/video"
}

// Mocked Telemetry Functions (replace with real device communication as needed)
func fetchSensorData(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"temperature": 25.1,
		"humidity":    40.0,
	}
}
func fetchAIResults(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"object_detection": []map[string]interface{}{
			{"label": "person", "confidence": 0.99},
		},
	}
}
func fetchCustomDeviceData(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"custom_metric": 123,
	}
}

// Mock OTA update procedure
func performOTAUpdate(req OTAUpdateRequest) {
	// Simulate OTA update (replace with real implementation)
	time.Sleep(2 * time.Second)
	log.Printf("OTA update triggered: %+v", req)
}

// Mock device control procedure
func performDeviceControl(req ControlCommandRequest) map[string]interface{} {
	// Simulate device control (replace with real implementation)
	log.Printf("Device control command: %+v", req)
	return map[string]interface{}{
		"status":  "Command executed",
		"command": req.Command,
	}
}