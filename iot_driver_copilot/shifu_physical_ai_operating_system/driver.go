package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// TelemetryData aggregates telemetry, sensor, video, and AI inference data.
type TelemetryData struct {
	Timestamp      time.Time              `json:"timestamp"`
	Telemetry      map[string]interface{} `json:"telemetry"`
	Sensor         map[string]interface{} `json:"sensor"`
	AIInference    map[string]interface{} `json:"ai_inference"`
	VideoStreamURL string                 `json:"video_stream_url,omitempty"`
}

// OTAUpdateRequest represents OTA update parameters.
type OTAUpdateRequest struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum,omitempty"`
}

// ControlCommand represents remote control command parameters.
type ControlCommand struct {
	Command string                 `json:"command"`
	Params  map[string]interface{} `json:"params"`
}

// DeviceClient simulates interaction with the physical AI OS.
type DeviceClient struct {
	Host string
	Port int
	APIToken string
}

func NewDeviceClientFromEnv() *DeviceClient {
	host := os.Getenv("DEVICE_HOST")
	portStr := os.Getenv("DEVICE_PORT")
	port, _ := strconv.Atoi(portStr)
	apiToken := os.Getenv("DEVICE_API_TOKEN")
	return &DeviceClient{
		Host: host,
		Port: port,
		APIToken: apiToken,
	}
}

// FetchTelemetry connects to the device and fetches telemetry/sensor/AI data.
func (dc *DeviceClient) FetchTelemetry(ctx context.Context) (*TelemetryData, error) {
	url := "http://" + dc.Host + ":" + strconv.Itoa(dc.Port) + "/api/telemetry"
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	if dc.APIToken != "" {
		req.Header.Set("Authorization", "Bearer " + dc.APIToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data TelemetryData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ProxyVideoStream connects to the device's video endpoint and proxies to HTTP.
func (dc *DeviceClient) ProxyVideoStream(w http.ResponseWriter, r *http.Request) {
	videoPath := os.Getenv("DEVICE_VIDEO_PATH")
	if videoPath == "" {
		videoPath = "/api/video/stream"
	}
	url := "http://" + dc.Host + ":" + strconv.Itoa(dc.Port) + videoPath
	req, _ := http.NewRequestWithContext(r.Context(), "GET", url, nil)
	if dc.APIToken != "" {
		req.Header.Set("Authorization", "Bearer " + dc.APIToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Unable to connect to video stream", 502)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// OTAUpdate proxies OTA request to device.
func (dc *DeviceClient) OTAUpdate(ctx context.Context, reqData *OTAUpdateRequest) (map[string]interface{}, error) {
	url := "http://" + dc.Host + ":" + strconv.Itoa(dc.Port) + "/api/ota"
	body, _ := json.Marshal(reqData)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if dc.APIToken != "" {
		req.Header.Set("Authorization", "Bearer " + dc.APIToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// SendControlCommand proxies control commands to device.
func (dc *DeviceClient) SendControlCommand(ctx context.Context, cmd *ControlCommand) (map[string]interface{}, error) {
	url := "http://" + dc.Host + ":" + strconv.Itoa(dc.Port) + "/api/control"
	body, _ := json.Marshal(cmd)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if dc.APIToken != "" {
		req.Header.Set("Authorization", "Bearer " + dc.APIToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func main() {
	serverHost := os.Getenv("SERVER_HOST")
	if serverHost == "" {
		serverHost = "0.0.0.0"
	}
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	deviceClient := NewDeviceClientFromEnv()

	http.HandleFunc("/telemetry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		query := r.URL.Query()
		if query.Get("video") == "true" {
			// Proxy raw video stream directly, HTTP-muxed from device
			deviceClient.ProxyVideoStream(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		data, err := deviceClient.FetchTelemetry(ctx)
		if err != nil {
			http.Error(w, "Failed to fetch telemetry: "+err.Error(), 502)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})

	http.HandleFunc("/ota", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		var otaReq OTAUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&otaReq); err != nil {
			http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		result, err := deviceClient.OTAUpdate(ctx, &otaReq)
		if err != nil {
			http.Error(w, "OTA update failed: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	http.HandleFunc("/control", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		var cmd ControlCommand
		if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
			http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		result, err := deviceClient.SendControlCommand(ctx, &cmd)
		if err != nil {
			http.Error(w, "Control command failed: "+err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	log.Printf("Shifu PAIOs HTTP Driver listening on %s:%s", serverHost, serverPort)
	log.Fatal(http.ListenAndServe(serverHost+":"+serverPort, nil))
}