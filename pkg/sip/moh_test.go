package sip

import (
	"testing"
	"time"
)

func TestNewMOHManager(t *testing.T) {
	tests := []struct {
		name     string
		cfg      MOHConfig
		wantPath string
	}{
		{
			name:     "default audio path",
			cfg:      MOHConfig{Enabled: true},
			wantPath: "/var/lib/gosip/moh/default.wav",
		},
		{
			name:     "custom audio path",
			cfg:      MOHConfig{Enabled: true, AudioPath: "/custom/path.wav"},
			wantPath: "/custom/path.wav",
		},
		{
			name:     "disabled MOH",
			cfg:      MOHConfig{Enabled: false},
			wantPath: "/var/lib/gosip/moh/default.wav",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewMOHManager(tt.cfg)
			if mgr == nil {
				t.Fatal("NewMOHManager returned nil")
			}
			status := mgr.GetStatus()
			if status.AudioPath != tt.wantPath {
				t.Errorf("AudioPath = %q, want %q", status.AudioPath, tt.wantPath)
			}
			if status.Enabled != tt.cfg.Enabled {
				t.Errorf("Enabled = %v, want %v", status.Enabled, tt.cfg.Enabled)
			}
		})
	}
}

func TestMOHManager_StartStop(t *testing.T) {
	mgr := NewMOHManager(MOHConfig{Enabled: true})
	session := &CallSession{
		CallID: "test-call-moh",
		State:  CallStateHeld,
	}

	// Start MOH
	err := mgr.Start("test-call-moh", session)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify active
	if !mgr.IsActive("test-call-moh") {
		t.Error("IsActive() = false, want true")
	}
	if mgr.GetActiveCount() != 1 {
		t.Errorf("GetActiveCount() = %d, want 1", mgr.GetActiveCount())
	}

	// Stop MOH
	mgr.Stop("test-call-moh")

	// Give goroutine time to stop
	time.Sleep(50 * time.Millisecond)

	// Verify stopped
	if mgr.IsActive("test-call-moh") {
		t.Error("IsActive() = true after Stop(), want false")
	}
	if mgr.GetActiveCount() != 0 {
		t.Errorf("GetActiveCount() = %d after Stop(), want 0", mgr.GetActiveCount())
	}
}

func TestMOHManager_DisabledMode(t *testing.T) {
	mgr := NewMOHManager(MOHConfig{Enabled: false})
	session := &CallSession{
		CallID: "test-disabled",
		State:  CallStateHeld,
	}

	// Start should not error but also not start stream
	err := mgr.Start("test-disabled", session)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Should not be active since MOH is disabled
	if mgr.IsActive("test-disabled") {
		t.Error("IsActive() = true when MOH disabled, want false")
	}
}

func TestMOHManager_DuplicateStart(t *testing.T) {
	mgr := NewMOHManager(MOHConfig{Enabled: true})
	session := &CallSession{
		CallID: "test-dup",
		State:  CallStateHeld,
	}

	// Start twice
	mgr.Start("test-dup", session)
	mgr.Start("test-dup", session)

	// Should only have one stream
	if mgr.GetActiveCount() != 1 {
		t.Errorf("GetActiveCount() = %d after duplicate start, want 1", mgr.GetActiveCount())
	}

	// Cleanup
	mgr.Stop("test-dup")
}

func TestMOHManager_StopNonExistent(t *testing.T) {
	mgr := NewMOHManager(MOHConfig{Enabled: true})

	// Should not panic
	mgr.Stop("non-existent-call")
}

func TestMOHManager_StopAll(t *testing.T) {
	mgr := NewMOHManager(MOHConfig{Enabled: true})

	sessions := []string{"call-1", "call-2", "call-3"}
	for _, callID := range sessions {
		mgr.Start(callID, &CallSession{CallID: callID, State: CallStateHeld})
	}

	if mgr.GetActiveCount() != 3 {
		t.Errorf("GetActiveCount() = %d, want 3", mgr.GetActiveCount())
	}

	mgr.StopAll()

	// Give goroutines time to stop
	time.Sleep(50 * time.Millisecond)

	if mgr.GetActiveCount() != 0 {
		t.Errorf("GetActiveCount() = %d after StopAll(), want 0", mgr.GetActiveCount())
	}
}

func TestMOHManager_EnableDisable(t *testing.T) {
	mgr := NewMOHManager(MOHConfig{Enabled: false})

	if mgr.IsEnabled() {
		t.Error("IsEnabled() = true, want false")
	}

	mgr.Enable(true)

	if !mgr.IsEnabled() {
		t.Error("IsEnabled() = false after Enable(true), want true")
	}

	mgr.Enable(false)

	if mgr.IsEnabled() {
		t.Error("IsEnabled() = true after Enable(false), want false")
	}
}

func TestMOHManager_SetAudioPath(t *testing.T) {
	mgr := NewMOHManager(MOHConfig{Enabled: true})

	mgr.SetAudioPath("/new/path/moh.wav")

	status := mgr.GetStatus()
	if status.AudioPath != "/new/path/moh.wav" {
		t.Errorf("AudioPath = %q, want %q", status.AudioPath, "/new/path/moh.wav")
	}
}

func TestMOHManager_GetStatus(t *testing.T) {
	mgr := NewMOHManager(MOHConfig{Enabled: true, AudioPath: "/test/path.wav"})
	session := &CallSession{CallID: "status-test", State: CallStateHeld}
	mgr.Start("status-test", session)

	status := mgr.GetStatus()

	if !status.Enabled {
		t.Error("Status.Enabled = false, want true")
	}
	if status.AudioPath != "/test/path.wav" {
		t.Errorf("Status.AudioPath = %q, want %q", status.AudioPath, "/test/path.wav")
	}
	if status.ActiveCount != 1 {
		t.Errorf("Status.ActiveCount = %d, want 1", status.ActiveCount)
	}

	// Cleanup
	mgr.Stop("status-test")
}

func TestMOHManager_GenerateSilence(t *testing.T) {
	mgr := NewMOHManager(MOHConfig{Enabled: true})
	silence := mgr.generateSilence()

	// Should be 1 second of silence at 8kHz
	if len(silence) != 8000 {
		t.Errorf("generateSilence() length = %d, want 8000", len(silence))
	}

	// PCMU silence is 0xFF
	for i, b := range silence {
		if b != 0xFF {
			t.Errorf("generateSilence()[%d] = %d, want 0xFF", i, b)
			break
		}
	}
}

func TestRTPPacket_Serialize(t *testing.T) {
	packet := CreateRTPPacket(PayloadTypePCMU, 1234, 5678, 0x12345678, []byte{0x01, 0x02, 0x03})

	if packet.Version != 2 {
		t.Errorf("Version = %d, want 2", packet.Version)
	}
	if packet.PayloadType != PayloadTypePCMU {
		t.Errorf("PayloadType = %d, want %d", packet.PayloadType, PayloadTypePCMU)
	}
	if packet.SequenceNumber != 1234 {
		t.Errorf("SequenceNumber = %d, want 1234", packet.SequenceNumber)
	}

	// Serialize and verify header structure
	data := packet.Serialize()

	// Minimum header is 12 bytes + payload
	if len(data) != 12+3 {
		t.Errorf("Serialize() length = %d, want 15", len(data))
	}

	// Check version (first 2 bits of first byte should be 2)
	if (data[0] >> 6) != 2 {
		t.Errorf("Serialized version = %d, want 2", data[0]>>6)
	}

	// Check payload type (7 bits of second byte)
	if data[1]&0x7F != PayloadTypePCMU {
		t.Errorf("Serialized payload type = %d, want %d", data[1]&0x7F, PayloadTypePCMU)
	}

	// Check payload
	if data[12] != 0x01 || data[13] != 0x02 || data[14] != 0x03 {
		t.Errorf("Payload not serialized correctly")
	}
}

func TestPayloadTypeConstants(t *testing.T) {
	if PayloadTypePCMU != 0 {
		t.Errorf("PayloadTypePCMU = %d, want 0", PayloadTypePCMU)
	}
	if PayloadTypePCMA != 8 {
		t.Errorf("PayloadTypePCMA = %d, want 8", PayloadTypePCMA)
	}
	if PayloadTypeG729 != 18 {
		t.Errorf("PayloadTypeG729 = %d, want 18", PayloadTypeG729)
	}
}
