package config

import (
	"fmt"
	"os"
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
	FPS             int
	SegmentDuration time.Duration
	MaxSegments     int
	Backend         CaptureBackend
	Display         string
	Hotkey          string
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
		VideoSize:       envString("CLIPER_VIDEO_SIZE", "1920x1080"),
		FPS:             envInt("CLIPER_FPS", 60),
		SegmentDuration: 5 * time.Second,
		MaxSegments:     24,
		Backend:         CaptureBackend(strings.ToLower(envString("CLIPER_BACKEND", "x11"))),
		Display:         envString("CLIPER_DISPLAY", envString("DISPLAY", ":0.0")),
		Hotkey:          strings.ToUpper(envString("CLIPER_HOTKEY", "F8")),
		RestartDelay:    2 * time.Second,
		CleanupInterval: 10 * time.Second,
	}

	if cfg.FPS <= 0 {
		return Config{}, fmt.Errorf("CLIPER_FPS must be greater than zero")
	}
	if cfg.MaxSegments <= 0 {
		return Config{}, fmt.Errorf("max segments must be greater than zero")
	}
	if cfg.SegmentDuration <= 0 {
		return Config{}, fmt.Errorf("segment duration must be greater than zero")
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

func isSupportedBackend(backend CaptureBackend) bool {
	switch backend {
	case BackendX11, BackendPipeWire, BackendKMSGrab:
		return true
	default:
		return false
	}
}
