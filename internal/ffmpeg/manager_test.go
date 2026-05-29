package ffmpeg

import (
	"reflect"
	"testing"
	"time"

	"github.com/example/cliper/internal/config"
)

func TestX11InputArgsUseConfiguredSizeDisplayAndOffset(t *testing.T) {
	manager := NewManager(config.Config{
		Backend:       config.BackendX11,
		FPS:           30,
		VideoSize:     "1366x768",
		Display:       ":0.0",
		CaptureOffset: "+10,20",
	})

	want := []string{"-f", "x11grab", "-framerate", "30", "-video_size", "1366x768", "-i", ":0.0+10,20"}
	if got := manager.inputArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("inputArgs() = %#v, want %#v", got, want)
	}
}

func TestRecordArgsUseConfiguredEncodingAndSegmentOptions(t *testing.T) {
	manager := NewManager(config.Config{
		Backend:         config.BackendX11,
		FPS:             60,
		VideoSize:       "1366x768",
		Display:         ":0",
		CaptureOffset:   "+0,0",
		VideoCodec:      "libx264",
		VideoPreset:     "fast",
		VideoTune:       "",
		VideoCRF:        "23",
		PixelFormat:     "yuv420p",
		SegmentDuration: 3 * time.Second,
		MaxSegments:     40,
		SegmentListPath: "/tmp/segments.ffconcat",
		SegmentName:     "/tmp/segment_%Y%m%d_%H%M%S.ts",
	})

	args := manager.recordArgs()
	assertContainsSequence(t, args, []string{"-c:v", "libx264"})
	assertContainsSequence(t, args, []string{"-preset", "fast"})
	assertContainsSequence(t, args, []string{"-crf", "23"})
	assertContainsSequence(t, args, []string{"-pix_fmt", "yuv420p"})
	assertContainsSequence(t, args, []string{"-segment_time", "3"})
	assertContainsSequence(t, args, []string{"-segment_list_size", "40"})
	assertNotContains(t, args, "-tune")
}

func assertContainsSequence(t *testing.T, args, sequence []string) {
	t.Helper()
	for i := 0; i <= len(args)-len(sequence); i++ {
		if reflect.DeepEqual(args[i:i+len(sequence)], sequence) {
			return
		}
	}
	t.Fatalf("args %#v do not contain sequence %#v", args, sequence)
}

func assertNotContains(t *testing.T, args []string, value string) {
	t.Helper()
	for _, arg := range args {
		if arg == value {
			t.Fatalf("args %#v unexpectedly contain %q", args, value)
		}
	}
}
