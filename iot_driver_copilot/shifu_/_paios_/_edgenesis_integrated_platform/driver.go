package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DeviceStatus struct {
	Uptime        string                 `json:"uptime"`
	DeviceStatus  string                 `json:"device_status"`
	DigitalTwin   map[string]interface{} `json:"digital_twin"`
	Health        string                 `json:"health"`
	Telemetry     map[string]interface{} `json:"telemetry"`
	LastUpdated   string                 `json:"last_updated"`
}

type ControlRequest struct {
	Command string                 `json:"command"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type DeployRequest struct {
	ModelName   string                 `json:"model_name"`
	Version     string                 `json:"version"`
	Config      map[string]interface{} `json:"config"`
}

type DeployResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ControlResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

var (
	serverHost   = getEnv("SERVER_HOST", "0.0.0.0")
	serverPort   = getEnv("SERVER_PORT", "8080")
	videoProto   = getEnv("VIDEO_PROTOCOL", "udp") // Only "udp" supported in this driver for raw video stream
	videoAddr    = getEnv("VIDEO_STREAM_ADDR", "")
	videoPort    = getEnv("VIDEO_STREAM_PORT", "")
	videoCodec   = getEnv("VIDEO_CODEC", "mjpeg") // Only "mjpeg" supported for browser
	statusPath   = getEnv("STATUS_PATH", "/status")
	videoPath    = getEnv("VIDEO_PATH", "/video")
	controlPath  = getEnv("CONTROL_PATH", "/control")
	deployPath   = getEnv("DEPLOY_PATH", "/deploy")
)

// Helper: get env var with fallback
func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

// Video stream proxy over HTTP (UDP MJPEG -> HTTP multipart/x-mixed-replace)
func videoHandler(w http.ResponseWriter, r *http.Request) {
	if videoProto != "udp" || videoAddr == "" || videoPort == "" || videoCodec != "mjpeg" {
		http.Error(w, "Video stream not configured or unsupported protocol/codec", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	ctx := r.Context()
	conn, err := net.ListenPacket("udp", net.JoinHostPort(serverHost, videoPort))
	if err != nil {
		http.Error(w, "Failed to bind UDP port: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer conn.Close()

	var mu sync.Mutex
	var lastFrame []byte
	go func() {
		buf := make([]byte, 65536)
		for {
			n, _, err := conn.ReadFrom(buf)
			if err != nil {
				break
			}
			mu.Lock()
			lastFrame = append([]byte{}, buf[:n]...)
			mu.Unlock()
		}
	}()

	ticker := time.NewTicker(40 * time.Millisecond) // ~25fps
	defer ticker.Stop()
	boundary := "--frame"
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mu.Lock()
			frame := lastFrame
			mu.Unlock()
			if len(frame) == 0 {
				continue
			}
			// Assume frame is a JPEG image over UDP (MJPEG streaming)
			_, _ = w.Write([]byte(boundary + "\r\n"))
			_, _ = w.Write([]byte("Content-Type: image/jpeg\r\n"))
			_, _ = w.Write([]byte("Content-Length: " + strconv.Itoa(len(frame)) + "\r\n\r\n"))
			_, _ = w.Write(frame)
			_, _ = w.Write([]byte("\r\n"))
			flusher.Flush()
		}
	}
}

// Simulated AI model deployment
func deployHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	resp := DeployResponse{
		Status:  "success",
		Message: "Model " + req.ModelName + " version " + req.Version + " deployed.",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Simulated device control
func controlHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req ControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	var status, msg string
	switch strings.ToLower(req.Command) {
	case "start":
		status = "started"
		msg = "Device started"
	case "stop":
		status = "stopped"
		msg = "Device stopped"
	case "restart":
		status = "restarted"
		msg = "Device restarted"
	case "discover":
		status = "discovering"
		msg = "Device discovery started"
	default:
		status = "unknown"
		msg = "Unknown command"
	}
	resp := ControlResponse{
		Status:  status,
		Message: msg,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Simulated device status/telemetry
func statusHandler(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(startTime).String()
	status := DeviceStatus{
		Uptime:       uptime,
		DeviceStatus: "online",
		DigitalTwin: map[string]interface{}{
			"temperature": 42.3,
			"location":    "edge node 1",
		},
		Health: "healthy",
		Telemetry: map[string]interface{}{
			"cpu":    21.5,
			"memory": 58.7,
		},
		LastUpdated: time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

var startTime = time.Now()

func main() {
	http.HandleFunc(videoPath, videoHandler)
	http.HandleFunc(deployPath, deployHandler)
	http.HandleFunc(controlPath, controlHandler)
	http.HandleFunc(statusPath, statusHandler)
	addr := net.JoinHostPort(serverHost, serverPort)
	log.Printf("Starting driver HTTP server on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}