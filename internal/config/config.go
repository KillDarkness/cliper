package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type CaptureBackend string

const (
	BackendX11      CaptureBackend = "x11"
	BackendPipeWire CaptureBackend = "pipewire"
	BackendKMSGrab  CaptureBackend = "kmsgrab"
)

type Config struct {
	FFmpegPath      string
	BufferDir       string
	ClipsDir        string
	VideoSize       string
	AutoVideoSize   bool
	FPS             int
	SegmentDuration time.Duration
	MaxSegments     int
	Backend         CaptureBackend
	Display         string
	CaptureOffset   string
	PipeWireNode    string
	Hotkey          string
	VideoCodec      string
	EncoderPreset   string
	CRF             int
	PixelFormat     string
	DrawMouse       bool
	SegmentListPath string
	SegmentName     string
	RestartDelay    time.Duration
	CleanupInterval time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		FFmpegPath:      envString("CLIPER_FFMPEG", "ffmpeg"),
		BufferDir:       envString("CLIPER_BUFFER_DIR", "buffer"),
		ClipsDir:        envString("CLIPER_CLIPS_DIR", "clips"),
		FPS:             envInt("CLIPER_FPS", 60),
		SegmentDuration: envDurationSeconds("CLIPER_SEGMENT_SECONDS", 5*time.Second),
		MaxSegments:     envInt("CLIPER_MAX_SEGMENTS", 24),
		Backend:         CaptureBackend(strings.ToLower(envString("CLIPER_BACKEND", "x11"))),
		Display:         envString("CLIPER_DISPLAY", envString("DISPLAY", ":0.0")),
		CaptureOffset:   normalizeOffset(envString("CLIPER_CAPTURE_OFFSET", "0,0")),
		PipeWireNode:    envString("CLIPER_PIPEWIRE_NODE", "0"),
		Hotkey:          strings.ToUpper(envString("CLIPER_HOTKEY", "F8")),
		VideoCodec:      envString("CLIPER_VIDEO_CODEC", "libx264"),
		EncoderPreset:   envString("CLIPER_PRESET", "veryfast"),
		CRF:             envInt("CLIPER_CRF", 23),
		PixelFormat:     envString("CLIPER_PIXEL_FORMAT", "yuv420p"),
		DrawMouse:       envBool("CLIPER_DRAW_MOUSE", true),
		RestartDelay:    envDurationSeconds("CLIPER_RESTART_DELAY_SECONDS", 2*time.Second),
		CleanupInterval: envDurationSeconds("CLIPER_CLEANUP_INTERVAL_SECONDS", 10*time.Second),
	}

	videoSizeEnv := strings.TrimSpace(os.Getenv("CLIPER_VIDEO_SIZE"))
	if videoSizeEnv == "" || strings.EqualFold(videoSizeEnv, "auto") {
		cfg.AutoVideoSize = true
		detected, err := detectVideoSize(cfg)
		if err == nil {
			cfg.VideoSize = detected
		} else {
			cfg.VideoSize = "1920x1080"
		}
	} else {
		cfg.VideoSize = videoSizeEnv
	}

	if cfg.FPS <= 0 {
		return Config{}, fmt.Errorf("CLIPER_FPS must be greater than zero")
	}
	if cfg.MaxSegments <= 0 {
		return Config{}, fmt.Errorf("max segments must be greater than zero")
	}
	if cfg.SegmentDuration <= 0 {
		return Config{}, fmt.Errorf("CLIPER_SEGMENT_SECONDS must be greater than zero")
	}
	if cfg.RestartDelay <= 0 {
		return Config{}, fmt.Errorf("CLIPER_RESTART_DELAY_SECONDS must be greater than zero")
	}
	if cfg.CleanupInterval <= 0 {
		return Config{}, fmt.Errorf("CLIPER_CLEANUP_INTERVAL_SECONDS must be greater than zero")
	}
	if cfg.CRF < 0 || cfg.CRF > 51 {
		return Config{}, fmt.Errorf("CLIPER_CRF must be between 0 and 51")
	}
	if !isValidVideoSize(cfg.VideoSize) {
		return Config{}, fmt.Errorf("CLIPER_VIDEO_SIZE must use WIDTHxHEIGHT format, got %q", cfg.VideoSize)
	}
	if !isSupportedBackend(cfg.Backend) {
		return Config{}, fmt.Errorf("unsupported CLIPER_BACKEND %q; use x11, pipewire, or kmsgrab", cfg.Backend)
	}

	bufferDir, err := filepath.Abs(cfg.BufferDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve buffer directory: %w", err)
	}
	clipsDir, err := filepath.Abs(cfg.ClipsDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve clips directory: %w", err)
	}
	cfg.BufferDir = bufferDir
	cfg.ClipsDir = clipsDir
	cfg.SegmentListPath = filepath.Join(cfg.BufferDir, "segments.ffconcat")
	cfg.SegmentName = filepath.Join(cfg.BufferDir, "segment_%Y%m%d_%H%M%S.ts")

	if err := os.MkdirAll(cfg.BufferDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create buffer directory: %w", err)
	}
	if err := os.MkdirAll(cfg.ClipsDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create clips directory: %w", err)
	}

	return cfg, nil
}

func (c Config) SegmentSeconds() int {
	return int(c.SegmentDuration / time.Second)
}

func (c Config) ClipDuration() time.Duration {
	return c.SegmentDuration * time.Duration(c.MaxSegments)
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDurationSeconds(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return time.Duration(parsed) * time.Second
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func detectVideoSize(cfg Config) (string, error) {
	if cfg.Backend != BackendX11 {
		return "", fmt.Errorf("automatic video size is only available for x11 backend")
	}
	if size, err := detectWithXDPYInfo(cfg.Display); err == nil {
		return size, nil
	}
	if size, err := detectWithXRandR(cfg.Display); err == nil {
		return size, nil
	}
	return "", fmt.Errorf("could not detect X11 monitor size")
}

func detectWithXDPYInfo(display string) (string, error) {
	args := []string{}
	if display != "" {
		args = append(args, "-display", display)
	}
	output, err := exec.Command("xdpyinfo", args...).Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "dimensions:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && isValidVideoSize(fields[1]) {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("xdpyinfo did not report dimensions")
}

func detectWithXRandR(display string) (string, error) {
	args := []string{"--current"}
	if display != "" {
		args = append([]string{"--display", display}, args...)
	}
	output, err := exec.Command("xrandr", args...).Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		for index, field := range fields {
			if field == "*" && index > 0 && isValidVideoSize(fields[index-1]) {
				return fields[index-1], nil
			}
		}
	}
	return "", fmt.Errorf("xrandr did not report active mode")
}

func isValidVideoSize(videoSize string) bool {
	width, height, ok := strings.Cut(strings.ToLower(strings.TrimSpace(videoSize)), "x")
	if !ok {
		return false
	}
	w, errW := strconv.Atoi(width)
	h, errH := strconv.Atoi(height)
	return errW == nil && errH == nil && w > 0 && h > 0
}

func normalizeOffset(offset string) string {
	offset = strings.TrimSpace(strings.TrimPrefix(offset, "+"))
	if offset == "" {
		return "0,0"
	}
	return offset
}

func isSupportedBackend(backend CaptureBackend) bool {
	switch backend {
	case BackendX11, BackendPipeWire, BackendKMSGrab:
		return true
	default:
		return false
	}
}
