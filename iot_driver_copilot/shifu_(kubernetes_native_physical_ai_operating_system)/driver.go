package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all environment variable configuration
type Config struct {
	DeviceIP           string
	MQTTBroker         string
	MQTTPort           int
	MQTTTopicTelemetry string
	MQTTTopicStatus    string
	MQTTTopicVideo     string
	MQTTTopicControl   string
	HTTPHost           string
	HTTPPort           int
	OTAFirmwareURL     string
}

func getEnv(key string, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func mustGetEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	num, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return num
}

func loadConfig() *Config {
	return &Config{
		DeviceIP:           getEnv("DEVICE_IP", "127.0.0.1"),
		MQTTBroker:         getEnv("MQTT_BROKER", "127.0.0.1"),
		MQTTPort:           mustGetEnvInt("MQTT_PORT", 1883),
		MQTTTopicTelemetry: getEnv("MQTT_TOPIC_TELEMETRY", "shifu/telemetry"),
		MQTTTopicStatus:    getEnv("MQTT_TOPIC_STATUS", "shifu/status"),
		MQTTTopicVideo:     getEnv("MQTT_TOPIC_VIDEO", "shifu/video"),
		MQTTTopicControl:   getEnv("MQTT_TOPIC_CONTROL", "shifu/control"),
		HTTPHost:           getEnv("HTTP_HOST", "0.0.0.0"),
		HTTPPort:           mustGetEnvInt("HTTP_PORT", 8080),
		OTAFirmwareURL:     getEnv("OTA_FIRMWARE_URL", ""),
	}
}

// --- MQTT Client Mock/Stub ---
// This is a minimal MQTT implementation for demo purposes only.
// In production, use a proper MQTT Go client (e.g., Eclipse Paho MQTT).
// But as per requirements, no third-party command execution is allowed.

type mqttMsg struct {
	Topic string
	Payload []byte
}

type mqttStub struct {
	// Simulate an MQTT broker by holding the last published messages per topic
	messages map[string][]byte
}

func newMQTTStub() *mqttStub {
	return &mqttStub{
		messages: map[string][]byte{
			"shifu/telemetry": []byte(`{"temperature": 25.2, "humidity": 51.3, "ts": "` + time.Now().Format(time.RFC3339Nano) + `"}`),
			"shifu/status":    []byte(`{"device":"PAIOS","status":"online","uptime":"` + fmt.Sprint(time.Now().Unix()-1710000000) + `"}`),
			"shifu/video":     []byte{}, // Filled dynamically
		},
	}
}

func (m *mqttStub) publish(topic string, payload []byte) {
	m.messages[topic] = payload
}

func (m *mqttStub) get(topic string) []byte {
	if data, ok := m.messages[topic]; ok {
		return data
	}
	return nil
}

// --- HTTP Handlers ---

func handleStatus(mqtt *mqttStub, config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := mqtt.get(config.MQTTTopicStatus)
		if status == nil {
			http.Error(w, "Status not available", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(status)
	}
}

func handleTelemetry(mqtt *mqttStub, config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		telemetry := mqtt.get(config.MQTTTopicTelemetry)
		if telemetry == nil {
			http.Error(w, "Telemetry not available", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(telemetry)
	}
}

// For demonstration: simulate MJPEG stream over HTTP (browser-compatible)
func handleVideo(mqtt *mqttStub, config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
		// Simulate a video stream by sending a static or periodically updated JPEG frame
		boundary := "--frame"
		for i := 0; i < 1000; i++ { // Limit stream for demo
			// In a real device, you would grab a frame from a camera or video topic
			img := getSimulatedJPEGFrame(i)
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(img))
			buf.Write(img)
			buf.WriteString("\r\n")
			_, err := w.Write(buf.Bytes())
			if err != nil {
				return
			}
			w.(http.Flusher).Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Simulate a JPEG frame (normally you'd get real image data)
func getSimulatedJPEGFrame(seq int) []byte {
	// This is a minimal JPEG image (1x1 pixel, all white)
	// For a real device, fetch a camera frame or decode from MQTT topic
	return []byte{
		0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x43, 0x00, 0x03, 0x02, 0x02, 0x03, 0x02, 0x02, 0x03, 0x03, 0x03,
		0x03, 0x04, 0x03, 0x03, 0x04, 0x05, 0x08, 0x05, 0x05, 0x04, 0x04, 0x05, 0x0A, 0x07, 0x07, 0x06,
		0x08, 0x0C, 0x0A, 0x0C, 0x0C, 0x0B, 0x0A, 0x0B, 0x0B, 0x0D, 0x0E, 0x12, 0x10, 0x0D, 0x0E, 0x11,
		0x0E, 0x0B, 0x0B, 0x10, 0x16, 0x10, 0x11, 0x13, 0x14, 0x15, 0x15, 0x15, 0x0C, 0x0F, 0x17, 0x18,
		0x16, 0x14, 0x18, 0x12, 0x14, 0x15, 0x14, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01, 0x00, 0x01,
		0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x14, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00,
		0x00, 0x3F, 0x00, 0xD2, 0xCF, 0x20, 0xFF, 0xD9,
	}
}

// OTA handler: Accepts a firmware binary via POST or triggers OTA from a URL
func handleOTA(mqtt *mqttStub, config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var firmware []byte
		var err error
		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "multipart/form-data") {
			err = r.ParseMultipartForm(10 << 20) // 10MB max
			if err != nil {
				http.Error(w, "Invalid multipart form", http.StatusBadRequest)
				return
			}
			file, _, err := r.FormFile("firmware")
			if err != nil {
				http.Error(w, "Missing firmware file", http.StatusBadRequest)
				return
			}
			defer file.Close()
			firmware, err = io.ReadAll(file)
			if err != nil {
				http.Error(w, "Failed to read firmware", http.StatusInternalServerError)
				return
			}
		} else if config.OTAFirmwareURL != "" {
			resp, err := http.Get(config.OTAFirmwareURL)
			if err != nil {
				http.Error(w, "Failed to fetch firmware from OTA_FIRMWARE_URL", http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()
			firmware, err = io.ReadAll(resp.Body)
			if err != nil {
				http.Error(w, "Failed to read fetched firmware", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "No firmware provided", http.StatusBadRequest)
			return
		}
		// Simulate OTA upgrade
		time.Sleep(2 * time.Second)
		mqtt.publish(config.MQTTTopicStatus, []byte(`{"device":"PAIOS","status":"upgraded","uptime":"`+fmt.Sprint(time.Now().Unix())+`"}`))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"result": "OTA upgrade successful",
			"size":   fmt.Sprintf("%d", len(firmware)),
		})
	}
}

// /control: Accepts JSON with control command, simulates execution
func handleControl(mqtt *mqttStub, config *Config) http.HandlerFunc {
	type ControlRequest struct {
		Command string `json:"command"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req ControlRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil || req.Command == "" {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		// Simulate device command
		switch strings.ToLower(req.Command) {
		case "start":
			mqtt.publish(config.MQTTTopicStatus, []byte(`{"device":"PAIOS","status":"running","ts":"`+time.Now().Format(time.RFC3339)+`"}`))
		case "stop":
			mqtt.publish(config.MQTTTopicStatus, []byte(`{"device":"PAIOS","status":"stopped","ts":"`+time.Now().Format(time.RFC3339)+`"}`))
		default:
			mqtt.publish(config.MQTTTopicStatus, []byte(`{"device":"PAIOS","status":"operating","command":"`+req.Command+`","ts":"`+time.Now().Format(time.RFC3339)+`"}`))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"result": "command executed"})
	}
}

// --- Main ---

func main() {
	config := loadConfig()
	mqtt := newMQTTStub()
	http.HandleFunc("/status", handleStatus(mqtt, config))
	http.HandleFunc("/telemetry", handleTelemetry(mqtt, config))
	http.HandleFunc("/video", handleVideo(mqtt, config))
	http.HandleFunc("/ota", handleOTA(mqtt, config))
	http.HandleFunc("/control", handleControl(mqtt, config))
	addr := fmt.Sprintf("%s:%d", config.HTTPHost, config.HTTPPort)
	log.Printf("Shifu PAIOS driver HTTP server started at %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}