package main

import "testing"

func TestDACLevelEndpointsAndClamp(t *testing.T) {
	for _, tc := range []struct {
		volume int
		want   int
	}{
		{-100, 0}, {-1, 0}, {0, 0}, {1, 130}, {32, 146},
		{100, 180}, {101, 180}, {1000, 180},
	} {
		if got := dacLevel(tc.volume); got != tc.want {
			t.Errorf("dacLevel(%d) = %d, want %d", tc.volume, got, tc.want)
		}
	}
}

func TestDACLevelMonotonicAudibleRange(t *testing.T) {
	previous := 0
	for volume := 1; volume <= 100; volume++ {
		got := dacLevel(volume)
		if got < 130 || got > 180 {
			t.Errorf("dacLevel(%d) = %d outside 130..180", volume, got)
		}
		if got < previous {
			t.Errorf("dacLevel(%d) = %d less than previous %d", volume, got, previous)
		}
		previous = got
	}
}
