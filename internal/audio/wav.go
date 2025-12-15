// Package audio provides audio file validation and processing utilities
package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// WAV format constants
const (
	// Supported sample rates for telephony
	SampleRate8kHz  = 8000
	SampleRate16kHz = 16000

	// Supported bits per sample
	BitsPerSample8  = 8
	BitsPerSample16 = 16

	// Supported channels
	ChannelsMono   = 1
	ChannelsStereo = 2

	// Max file size: 10MB
	MaxFileSize = 10 * 1024 * 1024

	// Min file duration: 1 second
	MinDurationSeconds = 1

	// Max file duration: 5 minutes
	MaxDurationSeconds = 300
)

// WAVValidationError represents a validation error with details
type WAVValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *WAVValidationError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Details)
	}
	return e.Message
}

// Validation error codes
const (
	ErrCodeInvalidFormat    = "INVALID_FORMAT"
	ErrCodeUnsupportedCodec = "UNSUPPORTED_CODEC"
	ErrCodeInvalidSampleRate = "INVALID_SAMPLE_RATE"
	ErrCodeInvalidBitDepth  = "INVALID_BIT_DEPTH"
	ErrCodeInvalidChannels  = "INVALID_CHANNELS"
	ErrCodeFileTooLarge     = "FILE_TOO_LARGE"
	ErrCodeFileTooShort     = "FILE_TOO_SHORT"
	ErrCodeFileTooLong      = "FILE_TOO_LONG"
	ErrCodeCorruptFile      = "CORRUPT_FILE"
)

// WAVHeader represents the parsed WAV file header
type WAVHeader struct {
	AudioFormat   uint16  // 1 = PCM, 3 = IEEE float, etc.
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	DataSize      uint32
}

// Duration returns the duration of the audio in seconds
func (h *WAVHeader) Duration() float64 {
	if h.ByteRate == 0 {
		return 0
	}
	return float64(h.DataSize) / float64(h.ByteRate)
}

// WAVValidationResult contains the validation outcome
type WAVValidationResult struct {
	Valid         bool       `json:"valid"`
	Header        *WAVHeader `json:"header,omitempty"`
	Duration      float64    `json:"duration,omitempty"`
	Error         *WAVValidationError `json:"error,omitempty"`
	Warnings      []string   `json:"warnings,omitempty"`
}

// ValidateWAV validates a WAV file from a reader
func ValidateWAV(r io.Reader, fileSize int64) *WAVValidationResult {
	result := &WAVValidationResult{}

	// Check file size
	if fileSize > MaxFileSize {
		result.Error = &WAVValidationError{
			Code:    ErrCodeFileTooLarge,
			Message: "File is too large",
			Details: fmt.Sprintf("Maximum size is %d MB, got %.2f MB", MaxFileSize/(1024*1024), float64(fileSize)/(1024*1024)),
		}
		return result
	}

	// Read RIFF header (12 bytes)
	riffHeader := make([]byte, 12)
	if _, err := io.ReadFull(r, riffHeader); err != nil {
		result.Error = &WAVValidationError{
			Code:    ErrCodeInvalidFormat,
			Message: "Failed to read RIFF header",
			Details: err.Error(),
		}
		return result
	}

	// Verify RIFF signature
	if string(riffHeader[0:4]) != "RIFF" {
		result.Error = &WAVValidationError{
			Code:    ErrCodeInvalidFormat,
			Message: "Not a valid WAV file",
			Details: "Missing RIFF signature",
		}
		return result
	}

	// Verify WAVE format
	if string(riffHeader[8:12]) != "WAVE" {
		result.Error = &WAVValidationError{
			Code:    ErrCodeInvalidFormat,
			Message: "Not a valid WAV file",
			Details: "Missing WAVE format identifier",
		}
		return result
	}

	// Parse chunks to find fmt and data
	header := &WAVHeader{}
	foundFmt := false
	foundData := false

	for !foundFmt || !foundData {
		// Read chunk header (8 bytes: 4 byte ID + 4 byte size)
		chunkHeader := make([]byte, 8)
		if _, err := io.ReadFull(r, chunkHeader); err != nil {
			if errors.Is(err, io.EOF) && foundFmt {
				// Some WAV files may not have data chunk at expected position
				break
			}
			result.Error = &WAVValidationError{
				Code:    ErrCodeCorruptFile,
				Message: "Failed to read chunk header",
				Details: err.Error(),
			}
			return result
		}

		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		switch chunkID {
		case "fmt ":
			if err := parseFmtChunk(r, chunkSize, header); err != nil {
				result.Error = &WAVValidationError{
					Code:    ErrCodeCorruptFile,
					Message: "Failed to parse fmt chunk",
					Details: err.Error(),
				}
				return result
			}
			foundFmt = true

		case "data":
			header.DataSize = chunkSize
			foundData = true
			// Don't read the actual data, just note its size

		default:
			// Skip unknown chunks
			if chunkSize > 0 {
				if _, err := io.CopyN(io.Discard, r, int64(chunkSize)); err != nil {
					// May reach EOF, which is ok if we have what we need
					if !errors.Is(err, io.EOF) {
						result.Error = &WAVValidationError{
							Code:    ErrCodeCorruptFile,
							Message: "Failed to skip chunk",
							Details: err.Error(),
						}
						return result
					}
				}
			}
		}

		// Add padding byte if chunk size is odd
		if chunkSize%2 == 1 {
			io.CopyN(io.Discard, r, 1)
		}
	}

	if !foundFmt {
		result.Error = &WAVValidationError{
			Code:    ErrCodeCorruptFile,
			Message: "Missing fmt chunk",
			Details: "WAV file does not contain format information",
		}
		return result
	}

	// Validate audio format (must be PCM = 1)
	if header.AudioFormat != 1 {
		formatName := getAudioFormatName(header.AudioFormat)
		result.Error = &WAVValidationError{
			Code:    ErrCodeUnsupportedCodec,
			Message: "Unsupported audio format",
			Details: fmt.Sprintf("Expected PCM (1), got %s (%d). Please convert to PCM WAV format.", formatName, header.AudioFormat),
		}
		return result
	}

	// Validate sample rate
	if header.SampleRate != SampleRate8kHz && header.SampleRate != SampleRate16kHz {
		result.Error = &WAVValidationError{
			Code:    ErrCodeInvalidSampleRate,
			Message: "Unsupported sample rate",
			Details: fmt.Sprintf("Expected 8000 Hz or 16000 Hz, got %d Hz. Please resample the audio.", header.SampleRate),
		}
		return result
	}

	// Validate bits per sample
	if header.BitsPerSample != BitsPerSample8 && header.BitsPerSample != BitsPerSample16 {
		result.Error = &WAVValidationError{
			Code:    ErrCodeInvalidBitDepth,
			Message: "Unsupported bit depth",
			Details: fmt.Sprintf("Expected 8-bit or 16-bit, got %d-bit. Please convert the audio.", header.BitsPerSample),
		}
		return result
	}

	// Validate channels (mono preferred, stereo allowed with warning)
	if header.NumChannels < ChannelsMono || header.NumChannels > ChannelsStereo {
		result.Error = &WAVValidationError{
			Code:    ErrCodeInvalidChannels,
			Message: "Unsupported channel configuration",
			Details: fmt.Sprintf("Expected mono (1) or stereo (2), got %d channels.", header.NumChannels),
		}
		return result
	}

	// Add warning for stereo (mono is preferred for telephony)
	if header.NumChannels == ChannelsStereo {
		result.Warnings = append(result.Warnings, "Stereo audio detected. Mono is recommended for telephony. The audio will work but may not play optimally.")
	}

	// Calculate and validate duration
	duration := header.Duration()
	if duration < MinDurationSeconds {
		result.Error = &WAVValidationError{
			Code:    ErrCodeFileTooShort,
			Message: "Audio is too short",
			Details: fmt.Sprintf("Minimum duration is %d second(s), got %.2f seconds.", MinDurationSeconds, duration),
		}
		return result
	}

	if duration > MaxDurationSeconds {
		result.Error = &WAVValidationError{
			Code:    ErrCodeFileTooLong,
			Message: "Audio is too long",
			Details: fmt.Sprintf("Maximum duration is %d seconds (5 minutes), got %.2f seconds.", MaxDurationSeconds, duration),
		}
		return result
	}

	// Add warning for non-standard sample rate for G.711
	if header.SampleRate == SampleRate16kHz {
		result.Warnings = append(result.Warnings, "16kHz sample rate detected. For best compatibility with G.711 codec, 8kHz is recommended.")
	}

	result.Valid = true
	result.Header = header
	result.Duration = duration

	return result
}

// parseFmtChunk parses the fmt chunk data
func parseFmtChunk(r io.Reader, size uint32, header *WAVHeader) error {
	// Minimum fmt chunk size is 16 bytes
	if size < 16 {
		return fmt.Errorf("fmt chunk too small: %d bytes", size)
	}

	fmtData := make([]byte, size)
	if _, err := io.ReadFull(r, fmtData); err != nil {
		return err
	}

	header.AudioFormat = binary.LittleEndian.Uint16(fmtData[0:2])
	header.NumChannels = binary.LittleEndian.Uint16(fmtData[2:4])
	header.SampleRate = binary.LittleEndian.Uint32(fmtData[4:8])
	header.ByteRate = binary.LittleEndian.Uint32(fmtData[8:12])
	header.BlockAlign = binary.LittleEndian.Uint16(fmtData[12:14])
	header.BitsPerSample = binary.LittleEndian.Uint16(fmtData[14:16])

	return nil
}

// getAudioFormatName returns a human-readable name for the audio format code
func getAudioFormatName(format uint16) string {
	switch format {
	case 1:
		return "PCM"
	case 2:
		return "Microsoft ADPCM"
	case 3:
		return "IEEE Float"
	case 6:
		return "A-law"
	case 7:
		return "μ-law"
	case 17:
		return "IMA ADPCM"
	case 85:
		return "MP3"
	case 0xFFFE:
		return "Extensible"
	default:
		return "Unknown"
	}
}

// ValidateWAVFile validates a WAV file from a path
func ValidateWAVFile(path string) (*WAVValidationResult, error) {
	// This is a placeholder - actual implementation would open the file
	// For the API endpoint, we use ValidateWAV with the uploaded file reader
	return nil, errors.New("use ValidateWAV with a reader instead")
}
