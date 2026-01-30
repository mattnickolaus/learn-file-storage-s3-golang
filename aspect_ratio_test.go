package main

import (
	"testing"
)

func TestGetAspectRatio(t *testing.T) {
	tests := []struct {
		name       string
		inputVideo string
		want       string
	}{
		{
			name:       "Test 1: 16:9 Video",
			inputVideo: "./samples/boots-video-horizontal.mp4",
			want:       "16:9",
		},
		{
			name:       "Test 2: 9:16 Video",
			inputVideo: "./samples/boots-video-vertical.mp4",
			want:       "9:16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := getVideoAspectRatio(tt.inputVideo)
			expected := tt.want

			if actual != expected {
				t.Errorf("got: %v; want: %v\n Error: %v", actual, expected, err)
			}

		})
	}
}
