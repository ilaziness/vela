package chrome

import (
	"testing"
	"time"
)

func TestClickTrackerDoubleClick(t *testing.T) {
	var tr ClickTracker

	if tr.DoubleClick(10, 10) {
		t.Fatal("first click reported as double")
	}
	if !tr.DoubleClick(12, 11) {
		t.Fatal("second click within interval/tolerance not reported as double")
	}
	// A matching pair resets the streak: the next click starts fresh.
	if tr.DoubleClick(12, 11) {
		t.Fatal("click right after a double reported as double again")
	}
	if !tr.DoubleClick(12, 11) {
		t.Fatal("pair after reset not reported as double")
	}
}

func TestClickTrackerRejectsDistantAndSlow(t *testing.T) {
	var tr ClickTracker
	tr.DoubleClick(10, 10)

	if tr.DoubleClick(10+doubleClickTolerance+1, 10) {
		t.Fatal("click beyond x tolerance reported as double")
	}
	tr.DoubleClick(10, 10)
	tr.lastTime = time.Now().Add(-doubleClickInterval - time.Millisecond)
	if tr.DoubleClick(10, 10) {
		t.Fatal("click beyond interval reported as double")
	}
}
