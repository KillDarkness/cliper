package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type CaptureBackend string

const (
	BackendX11      CaptureBackend = "x11"
	BackendPipeWire CaptureBackend = "pipewire"
	BackendKMSGrab  CaptureBackend = "kmsgrab"

	AutoVideoSize    = "auto"
	DefaultVideoSize = "1920x1080"
)

type Config struct {
	FFmpegPath      string
	BufferDir       string
	ClipsDir        string
	VideoSize       string
	VideoSizeSource string
	CaptureOffset   string
	FPS             int
	VideoCodec      string
	VideoPreset     string
	VideoTune       string
	VideoCRF        string
	PixelFormat     string
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
		VideoSize:       strings.ToLower(envString("CLIPER_VIDEO_SIZE", AutoVideoSize)),
		CaptureOffset:   envString("CLIPER_CAPTURE_OFFSET", "+0,0"),
		FPS:             envInt("CLIPER_FPS", 60),
		VideoCodec:      envString("CLIPER_VIDEO_CODEC", "libx264"),
		VideoPreset:     envString("CLIPER_VIDEO_PRESET", "veryfast"),
		VideoTune:       envString("CLIPER_VIDEO_TUNE", "zerolatency"),
		VideoCRF:        envString("CLIPER_VIDEO_CRF", ""),
		PixelFormat:     envString("CLIPER_PIXEL_FORMAT", "yuv420p"),
		SegmentDuration: envDuration("CLIPER_SEGMENT_DURATION", 5*time.Second),
		MaxSegments:     envInt("CLIPER_MAX_SEGMENTS", 24),
		Backend:         CaptureBackend(strings.ToLower(envString("CLIPER_BACKEND", "x11"))),
		Display:         envString("CLIPER_DISPLAY", envString("DISPLAY", ":0.0")),
		Hotkey:          strings.ToUpper(envString("CLIPER_HOTKEY", "F8")),
		RestartDelay:    envDuration("CLIPER_RESTART_DELAY", 2*time.Second),
		CleanupInterval: envDuration("CLIPER_CLEANUP_INTERVAL", 10*time.Second),
	}

	if cfg.FPS <= 0 {
		return Config{}, fmt.Errorf("CLIPER_FPS must be greater than zero")
	}
	if cfg.MaxSegments <= 0 {
		return Config{}, fmt.Errorf("CLIPER_MAX_SEGMENTS must be greater than zero")
	}
	if cfg.SegmentDuration <= 0 {
		return Config{}, fmt.Errorf("CLIPER_SEGMENT_DURATION must be greater than zero")
	}
	if cfg.RestartDelay <= 0 {
		return Config{}, fmt.Errorf("CLIPER_RESTART_DELAY must be greater than zero")
	}
	if cfg.CleanupInterval <= 0 {
		return Config{}, fmt.Errorf("CLIPER_CLEANUP_INTERVAL must be greater than zero")
	}
	if !isSupportedBackend(cfg.Backend) {
		return Config{}, fmt.Errorf("unsupported CLIPER_BACKEND %q; use x11, pipewire, or kmsgrab", cfg.Backend)
	}
	if err := validateVideoSize(&cfg); err != nil {
		return Config{}, err
	}
	if err := validateCaptureOffset(cfg.CaptureOffset); err != nil {
		return Config{}, err
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

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err == nil {
		return parsed
	}
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func validateVideoSize(cfg *Config) error {
	if strings.EqualFold(cfg.VideoSize, AutoVideoSize) {
		if cfg.Backend == BackendX11 {
			videoSize, err := DetectX11VideoSize(cfg.Display)
			if err == nil {
				cfg.VideoSize = videoSize
				cfg.VideoSizeSource = fmt.Sprintf("auto-detected from %s", cfg.Display)
				return nil
			}
		}
		cfg.VideoSize = DefaultVideoSize
		cfg.VideoSizeSource = "default fallback"
		return nil
	}
	if !isVideoSize(cfg.VideoSize) {
		return fmt.Errorf("CLIPER_VIDEO_SIZE must be %q or WIDTHxHEIGHT, got %q", AutoVideoSize, cfg.VideoSize)
	}
	cfg.VideoSizeSource = "CLIPER_VIDEO_SIZE"
	return nil
}

func validateCaptureOffset(offset string) error {
	if offset == "" {
		return nil
	}
	if !regexp.MustCompile(`^\+\d+,\d+$`).MatchString(offset) {
		return fmt.Errorf("CLIPER_CAPTURE_OFFSET must use +X,Y format, got %q", offset)
	}
	return nil
}

func DetectX11VideoSize(display string) (string, error) {
	if size, err := detectWithXDPYInfo(display); err == nil {
		return size, nil
	}
	if size, err := detectWithXRandR(display); err == nil {
		return size, nil
	}
	return "", fmt.Errorf("detect X11 display size")
}

func detectWithXDPYInfo(display string) (string, error) {
	output, err := runDisplayCommand(display, "xdpyinfo")
	if err != nil {
		return "", err
	}
	match := regexp.MustCompile(`dimensions:\s+(\d+x\d+)\s+pixels`).FindStringSubmatch(output)
	if len(match) != 2 {
		return "", fmt.Errorf("xdpyinfo dimensions not found")
	}
	return match[1], nil
}

func detectWithXRandR(display string) (string, error) {
	output, err := runDisplayCommand(display, "xrandr", "--current")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.Contains(line, "*") {
			continue
		}
		if isVideoSize(fields[0]) {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("xrandr active mode not found")
}

func runDisplayCommand(display, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	if display != "" {
		cmd.Env = append(cmd.Env, "DISPLAY="+display)
	}
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func isVideoSize(videoSize string) bool {
	return regexp.MustCompile(`^[1-9]\d*x[1-9]\d*$`).MatchString(strings.ToLower(videoSize))
}

func isSupportedBackend(backend CaptureBackend) bool {
	switch backend {
	case BackendX11, BackendPipeWire, BackendKMSGrab:
		return true
	default:
		return false
	}
}
