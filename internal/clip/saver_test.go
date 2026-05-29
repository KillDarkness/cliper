package clip

import (
	"reflect"
	"testing"
	"time"

	"github.com/example/cliper/internal/config"
)

func TestSegmentsForDurationKeepsLatestSegmentsRoundedUp(t *testing.T) {
	saver := NewSaver(config.Config{SegmentDuration: 5 * time.Second})
	segments := []string{"s1.ts", "s2.ts", "s3.ts", "s4.ts", "s5.ts"}

	got, err := saver.segmentsForDuration(segments, 11*time.Second)
	if err != nil {
		t.Fatalf("segmentsForDuration() error = %v", err)
	}

	want := []string{"s3.ts", "s4.ts", "s5.ts"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("segmentsForDuration() = %#v, want %#v", got, want)
	}
}

func TestSegmentsForDurationZeroKeepsAllSegments(t *testing.T) {
	saver := NewSaver(config.Config{SegmentDuration: 5 * time.Second})
	segments := []string{"s1.ts", "s2.ts"}

	got, err := saver.segmentsForDuration(segments, 0)
	if err != nil {
		t.Fatalf("segmentsForDuration() error = %v", err)
	}
	if !reflect.DeepEqual(got, segments) {
		t.Fatalf("segmentsForDuration() = %#v, want %#v", got, segments)
	}
}

func TestSegmentsForDurationRejectsNegativeDuration(t *testing.T) {
	saver := NewSaver(config.Config{SegmentDuration: 5 * time.Second})

	if _, err := saver.segmentsForDuration([]string{"s1.ts"}, -time.Second); err == nil {
		t.Fatal("segmentsForDuration() error = nil, want negative duration error")
	}
}
