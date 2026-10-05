package jshttp

import (
	"slices"
	"testing"
)

func TestLaunchArgsOmitSingleProcessOnWindows(t *testing.T) {
	t.Parallel()

	if slices.Contains(launchArgs("windows", false), "--single-process") {
		t.Fatal("--single-process must not be passed on Windows: Chromium exits on the first navigation")
	}
}

func TestLaunchArgsKeepSingleProcessOnOtherPlatforms(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{"linux", "darwin"} {
		if !slices.Contains(launchArgs(goos, false), "--single-process") {
			t.Errorf("expected --single-process on %s", goos)
		}
	}
}

func TestLaunchArgsDisableImages(t *testing.T) {
	t.Parallel()

	const flag = "--blink-settings=imagesEnabled=false"

	if slices.Contains(launchArgs("linux", false), flag) {
		t.Errorf("did not expect %s when images are enabled", flag)
	}

	if !slices.Contains(launchArgs("linux", true), flag) {
		t.Errorf("expected %s when images are disabled", flag)
	}
}
