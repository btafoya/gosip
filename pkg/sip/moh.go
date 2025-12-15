// Package sip provides Music on Hold functionality for GoSIP
package sip

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MOHManager manages Music on Hold streams
type MOHManager struct {
	mu           sync.RWMutex
	activeStreams map[string]*MOHStream
	audioPath    string
	enabled      bool
}

// MOHStream represents an active MOH stream for a call
type MOHStream struct {
	CallID    string
	Session   *CallSession
	StartedAt time.Time
	StopChan  chan struct{}
	AudioData []byte
}

// MOHConfig holds configuration for Music on Hold
type MOHConfig struct {
	Enabled   bool
	AudioPath string // Path to MOH audio file (WAV format)
}

// NewMOHManager creates a new Music on Hold manager
func NewMOHManager(cfg MOHConfig) *MOHManager {
	mgr := &MOHManager{
		activeStreams: make(map[string]*MOHStream),
		audioPath:     cfg.AudioPath,
		enabled:       cfg.Enabled,
	}

	// Set default audio path if not specified
	if mgr.audioPath == "" {
		mgr.audioPath = "/var/lib/gosip/moh/default.wav"
	}

	return mgr
}

// Start begins streaming MOH for a held call
func (m *MOHManager) Start(callID string, session *CallSession) error {
	if !m.enabled {
		slog.Debug("MOH disabled, not starting stream", "call_id", callID)
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already streaming
	if _, exists := m.activeStreams[callID]; exists {
		slog.Debug("MOH already active for call", "call_id", callID)
		return nil
	}

	// Load audio data
	audioData, err := m.loadAudioFile()
	if err != nil {
		slog.Warn("Failed to load MOH audio, using silence", "error", err)
		audioData = m.generateSilence()
	}

	stream := &MOHStream{
		CallID:    callID,
		Session:   session,
		StartedAt: time.Now(),
		StopChan:  make(chan struct{}),
		AudioData: audioData,
	}

	m.activeStreams[callID] = stream

	// Start streaming in background
	go m.streamAudio(stream)

	slog.Info("MOH started", "call_id", callID)
	return nil
}

// Stop ends MOH streaming for a call
func (m *MOHManager) Stop(callID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, exists := m.activeStreams[callID]
	if !exists {
		return
	}

	// Signal stream to stop
	close(stream.StopChan)
	delete(m.activeStreams, callID)

	slog.Info("MOH stopped",
		"call_id", callID,
		"duration", time.Since(stream.StartedAt).String(),
	)
}

// IsActive returns true if MOH is streaming for a call
func (m *MOHManager) IsActive(callID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.activeStreams[callID]
	return exists
}

// GetActiveCount returns the number of active MOH streams
func (m *MOHManager) GetActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.activeStreams)
}

// StopAll stops all active MOH streams
func (m *MOHManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for callID, stream := range m.activeStreams {
		close(stream.StopChan)
		delete(m.activeStreams, callID)
	}

	slog.Info("All MOH streams stopped")
}

// streamAudio handles the actual audio streaming
func (m *MOHManager) streamAudio(stream *MOHStream) {
	// In a full implementation, this would:
	// 1. Parse the audio file (WAV/MP3)
	// 2. Transcode to appropriate codec (PCMU/PCMA)
	// 3. Send RTP packets to the held party
	// 4. Loop the audio until stop is signaled

	// For now, this is a placeholder that simulates streaming
	ticker := time.NewTicker(20 * time.Millisecond) // 20ms RTP packet interval
	defer ticker.Stop()

	audioLen := len(stream.AudioData)
	if audioLen == 0 {
		audioLen = 160 // Minimum packet size
	}

	position := 0
	packetSize := 160 // 20ms at 8kHz

	for {
		select {
		case <-stream.StopChan:
			return
		case <-ticker.C:
			// Calculate packet to send
			endPos := position + packetSize
			if endPos > audioLen {
				// Loop back to start
				position = 0
				endPos = packetSize
			}

			// In full implementation, send RTP packet here
			// sendRTPPacket(stream.Session, stream.AudioData[position:endPos])

			position = endPos
		}
	}
}

// loadAudioFile loads the MOH audio file
func (m *MOHManager) loadAudioFile() ([]byte, error) {
	// Check if file exists
	if _, err := os.Stat(m.audioPath); os.IsNotExist(err) {
		return nil, err
	}

	file, err := os.Open(m.audioPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// If WAV file, skip header
	if filepath.Ext(m.audioPath) == ".wav" && len(data) > 44 {
		data = data[44:] // Skip WAV header
	}

	return data, nil
}

// generateSilence creates silent audio data
func (m *MOHManager) generateSilence() []byte {
	// Generate 1 second of silence at 8kHz (8000 samples)
	// PCMU silence is 0xFF (255)
	silence := make([]byte, 8000)
	for i := range silence {
		silence[i] = 0xFF
	}
	return silence
}

// SetAudioPath updates the MOH audio file path
func (m *MOHManager) SetAudioPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audioPath = path
}

// Enable enables or disables MOH
func (m *MOHManager) Enable(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
}

// IsEnabled returns whether MOH is enabled
func (m *MOHManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// MOHStatus represents the current MOH status for API responses
type MOHStatus struct {
	Enabled      bool   `json:"enabled"`
	AudioPath    string `json:"audio_path"`
	ActiveCount  int    `json:"active_count"`
}

// GetStatus returns the current MOH status
func (m *MOHManager) GetStatus() MOHStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return MOHStatus{
		Enabled:     m.enabled,
		AudioPath:   m.audioPath,
		ActiveCount: len(m.activeStreams),
	}
}

// RTPPacket represents an RTP packet for audio streaming
type RTPPacket struct {
	Version        uint8
	Padding        bool
	Extension      bool
	CSRCCount      uint8
	Marker         bool
	PayloadType    uint8
	SequenceNumber uint16
	Timestamp      uint32
	SSRC           uint32
	Payload        []byte
}

// CreateRTPPacket creates an RTP packet for audio data
func CreateRTPPacket(payloadType uint8, seqNum uint16, timestamp uint32, ssrc uint32, payload []byte) *RTPPacket {
	return &RTPPacket{
		Version:        2,
		Padding:        false,
		Extension:      false,
		CSRCCount:      0,
		Marker:         false,
		PayloadType:    payloadType,
		SequenceNumber: seqNum,
		Timestamp:      timestamp,
		SSRC:           ssrc,
		Payload:        payload,
	}
}

// Serialize converts RTP packet to bytes
func (p *RTPPacket) Serialize() []byte {
	header := make([]byte, 12)

	// First byte: V=2, P, X, CC
	header[0] = (p.Version << 6)
	if p.Padding {
		header[0] |= 0x20
	}
	if p.Extension {
		header[0] |= 0x10
	}
	header[0] |= (p.CSRCCount & 0x0F)

	// Second byte: M, PT
	header[1] = p.PayloadType & 0x7F
	if p.Marker {
		header[1] |= 0x80
	}

	// Sequence number (big endian)
	header[2] = byte(p.SequenceNumber >> 8)
	header[3] = byte(p.SequenceNumber)

	// Timestamp (big endian)
	header[4] = byte(p.Timestamp >> 24)
	header[5] = byte(p.Timestamp >> 16)
	header[6] = byte(p.Timestamp >> 8)
	header[7] = byte(p.Timestamp)

	// SSRC (big endian)
	header[8] = byte(p.SSRC >> 24)
	header[9] = byte(p.SSRC >> 16)
	header[10] = byte(p.SSRC >> 8)
	header[11] = byte(p.SSRC)

	// Combine header and payload
	packet := make([]byte, len(header)+len(p.Payload))
	copy(packet, header)
	copy(packet[12:], p.Payload)

	return packet
}

// Common audio codec payload types
const (
	PayloadTypePCMU = 0  // G.711 μ-law
	PayloadTypePCMA = 8  // G.711 A-law
	PayloadTypeG729 = 18 // G.729
)
