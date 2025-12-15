package audio

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// createValidWAVHeader creates a minimal valid WAV file header
func createValidWAVHeader(sampleRate, bitsPerSample, numChannels, dataSize uint32) []byte {
	buf := new(bytes.Buffer)

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize)) // File size - 8
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16)) // Chunk size
	binary.Write(buf, binary.LittleEndian, uint16(1))  // Audio format (PCM)
	binary.Write(buf, binary.LittleEndian, uint16(numChannels))
	binary.Write(buf, binary.LittleEndian, sampleRate)
	byteRate := sampleRate * numChannels * (bitsPerSample / 8)
	binary.Write(buf, binary.LittleEndian, byteRate)
	blockAlign := numChannels * (bitsPerSample / 8)
	binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))

	// data chunk
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, dataSize)

	// Add audio data (silence)
	for i := uint32(0); i < dataSize; i++ {
		buf.WriteByte(0xFF) // PCMU silence
	}

	return buf.Bytes()
}

func TestValidateWAV_ValidFile(t *testing.T) {
	// Create a valid 8kHz, 16-bit, mono WAV file with 2 seconds of audio
	// 2 seconds at 8kHz, 16-bit, mono = 8000 * 2 * 2 = 32000 bytes
	dataSize := uint32(32000)
	wavData := createValidWAVHeader(8000, 16, 1, dataSize)

	result := ValidateWAV(bytes.NewReader(wavData), int64(len(wavData)))

	if !result.Valid {
		t.Errorf("Expected valid WAV file, got error: %v", result.Error)
	}
	if result.Header == nil {
		t.Fatal("Expected header to be populated")
	}
	if result.Header.SampleRate != 8000 {
		t.Errorf("Expected sample rate 8000, got %d", result.Header.SampleRate)
	}
	if result.Header.BitsPerSample != 16 {
		t.Errorf("Expected 16 bits per sample, got %d", result.Header.BitsPerSample)
	}
	if result.Header.NumChannels != 1 {
		t.Errorf("Expected 1 channel, got %d", result.Header.NumChannels)
	}
	if result.Duration < 1.9 || result.Duration > 2.1 {
		t.Errorf("Expected duration ~2 seconds, got %f", result.Duration)
	}
}

func TestValidateWAV_Valid16kHz(t *testing.T) {
	// Create a valid 16kHz WAV file
	dataSize := uint32(32000) // 1 second at 16kHz, 16-bit, mono
	wavData := createValidWAVHeader(16000, 16, 1, dataSize)

	result := ValidateWAV(bytes.NewReader(wavData), int64(len(wavData)))

	if !result.Valid {
		t.Errorf("Expected valid WAV file, got error: %v", result.Error)
	}
	// Should have a warning about 16kHz
	if len(result.Warnings) == 0 {
		t.Error("Expected warning about 16kHz sample rate")
	}
}

func TestValidateWAV_StereoFile(t *testing.T) {
	// Create a stereo WAV file (should be valid but with warning)
	dataSize := uint32(32000)
	wavData := createValidWAVHeader(8000, 16, 2, dataSize)

	result := ValidateWAV(bytes.NewReader(wavData), int64(len(wavData)))

	if !result.Valid {
		t.Errorf("Expected valid WAV file, got error: %v", result.Error)
	}
	if len(result.Warnings) == 0 {
		t.Error("Expected warning about stereo audio")
	}
}

func TestValidateWAV_InvalidFormat_NotRIFF(t *testing.T) {
	data := []byte("This is not a WAV file")

	result := ValidateWAV(bytes.NewReader(data), int64(len(data)))

	if result.Valid {
		t.Error("Expected invalid result for non-WAV file")
	}
	if result.Error == nil || result.Error.Code != ErrCodeInvalidFormat {
		t.Errorf("Expected INVALID_FORMAT error, got: %v", result.Error)
	}
}

func TestValidateWAV_InvalidSampleRate(t *testing.T) {
	// Create WAV with unsupported sample rate (44100 Hz)
	dataSize := uint32(32000)
	wavData := createValidWAVHeader(44100, 16, 1, dataSize)

	result := ValidateWAV(bytes.NewReader(wavData), int64(len(wavData)))

	if result.Valid {
		t.Error("Expected invalid result for 44100 Hz sample rate")
	}
	if result.Error == nil || result.Error.Code != ErrCodeInvalidSampleRate {
		t.Errorf("Expected INVALID_SAMPLE_RATE error, got: %v", result.Error)
	}
}

func TestValidateWAV_InvalidBitDepth(t *testing.T) {
	// Create WAV with unsupported bit depth (24-bit)
	buf := new(bytes.Buffer)

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+1000))
	buf.WriteString("WAVE")

	// fmt chunk with 24-bit depth
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(buf, binary.LittleEndian, uint16(1)) // mono
	binary.Write(buf, binary.LittleEndian, uint32(8000))
	binary.Write(buf, binary.LittleEndian, uint32(24000)) // byteRate
	binary.Write(buf, binary.LittleEndian, uint16(3))     // blockAlign
	binary.Write(buf, binary.LittleEndian, uint16(24))    // 24-bit

	// data chunk
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(1000))
	for i := 0; i < 1000; i++ {
		buf.WriteByte(0)
	}

	wavData := buf.Bytes()
	result := ValidateWAV(bytes.NewReader(wavData), int64(len(wavData)))

	if result.Valid {
		t.Error("Expected invalid result for 24-bit audio")
	}
	if result.Error == nil || result.Error.Code != ErrCodeInvalidBitDepth {
		t.Errorf("Expected INVALID_BIT_DEPTH error, got: %v", result.Error)
	}
}

func TestValidateWAV_NonPCMFormat(t *testing.T) {
	buf := new(bytes.Buffer)

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+1000))
	buf.WriteString("WAVE")

	// fmt chunk with non-PCM format (3 = IEEE float)
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(3)) // IEEE float
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint32(8000))
	binary.Write(buf, binary.LittleEndian, uint32(16000))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(16))

	// data chunk
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(1000))
	for i := 0; i < 1000; i++ {
		buf.WriteByte(0)
	}

	wavData := buf.Bytes()
	result := ValidateWAV(bytes.NewReader(wavData), int64(len(wavData)))

	if result.Valid {
		t.Error("Expected invalid result for IEEE float format")
	}
	if result.Error == nil || result.Error.Code != ErrCodeUnsupportedCodec {
		t.Errorf("Expected UNSUPPORTED_CODEC error, got: %v", result.Error)
	}
}

func TestValidateWAV_FileTooLarge(t *testing.T) {
	data := []byte("test")

	// Simulate a file larger than MaxFileSize
	result := ValidateWAV(bytes.NewReader(data), MaxFileSize+1)

	if result.Valid {
		t.Error("Expected invalid result for oversized file")
	}
	if result.Error == nil || result.Error.Code != ErrCodeFileTooLarge {
		t.Errorf("Expected FILE_TOO_LARGE error, got: %v", result.Error)
	}
}

func TestValidateWAV_FileTooShort(t *testing.T) {
	// Create a WAV file with less than 1 second of audio
	// 0.5 seconds at 8kHz, 16-bit, mono = 8000 bytes
	dataSize := uint32(8000)
	wavData := createValidWAVHeader(8000, 16, 1, dataSize)

	result := ValidateWAV(bytes.NewReader(wavData), int64(len(wavData)))

	if result.Valid {
		t.Error("Expected invalid result for short audio")
	}
	if result.Error == nil || result.Error.Code != ErrCodeFileTooShort {
		t.Errorf("Expected FILE_TOO_SHORT error, got: %v", result.Error)
	}
}

func TestValidateWAV_FileTooLong(t *testing.T) {
	// Create a header indicating more than 5 minutes of audio
	// 6 minutes at 8kHz, 16-bit, mono = 8000 * 2 * 360 = 5,760,000 bytes
	dataSize := uint32(5760000)

	buf := new(bytes.Buffer)
	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(buf, binary.LittleEndian, uint16(1))
	binary.Write(buf, binary.LittleEndian, uint32(8000))
	binary.Write(buf, binary.LittleEndian, uint32(16000))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(16))

	// data chunk (just header, no actual data needed for validation)
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, dataSize)

	wavData := buf.Bytes()
	result := ValidateWAV(bytes.NewReader(wavData), int64(len(wavData)))

	if result.Valid {
		t.Error("Expected invalid result for long audio")
	}
	if result.Error == nil || result.Error.Code != ErrCodeFileTooLong {
		t.Errorf("Expected FILE_TOO_LONG error, got: %v", result.Error)
	}
}

func TestValidateWAV_8BitAudio(t *testing.T) {
	// Create a valid 8-bit WAV file
	// 2 seconds at 8kHz, 8-bit, mono = 8000 * 1 * 2 = 16000 bytes
	dataSize := uint32(16000)
	wavData := createValidWAVHeader(8000, 8, 1, dataSize)

	result := ValidateWAV(bytes.NewReader(wavData), int64(len(wavData)))

	if !result.Valid {
		t.Errorf("Expected valid 8-bit WAV file, got error: %v", result.Error)
	}
	if result.Header.BitsPerSample != 8 {
		t.Errorf("Expected 8 bits per sample, got %d", result.Header.BitsPerSample)
	}
}

func TestWAVHeader_Duration(t *testing.T) {
	tests := []struct {
		name     string
		header   WAVHeader
		expected float64
	}{
		{
			name: "1 second at 8kHz mono 16-bit",
			header: WAVHeader{
				ByteRate: 16000,
				DataSize: 16000,
			},
			expected: 1.0,
		},
		{
			name: "2 seconds at 16kHz stereo 16-bit",
			header: WAVHeader{
				ByteRate: 64000,
				DataSize: 128000,
			},
			expected: 2.0,
		},
		{
			name: "zero byte rate",
			header: WAVHeader{
				ByteRate: 0,
				DataSize: 16000,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.header.Duration()
			if got != tt.expected {
				t.Errorf("Duration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetAudioFormatName(t *testing.T) {
	tests := []struct {
		format   uint16
		expected string
	}{
		{1, "PCM"},
		{3, "IEEE Float"},
		{6, "A-law"},
		{7, "μ-law"},
		{85, "MP3"},
		{999, "Unknown"},
	}

	for _, tt := range tests {
		got := getAudioFormatName(tt.format)
		if got != tt.expected {
			t.Errorf("getAudioFormatName(%d) = %s, want %s", tt.format, got, tt.expected)
		}
	}
}

func TestWAVValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      WAVValidationError
		expected string
	}{
		{
			name: "with details",
			err: WAVValidationError{
				Code:    ErrCodeInvalidFormat,
				Message: "Invalid format",
				Details: "Missing RIFF header",
			},
			expected: "Invalid format: Missing RIFF header",
		},
		{
			name: "without details",
			err: WAVValidationError{
				Code:    ErrCodeInvalidFormat,
				Message: "Invalid format",
			},
			expected: "Invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %s, want %s", got, tt.expected)
			}
		})
	}
}
