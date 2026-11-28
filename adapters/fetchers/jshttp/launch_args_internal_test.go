package jshttp

import (
	"slices"
	"testing"
)

func TestLaunchArgsOmitSingleProcessOnWindowsAndMacOS(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{"windows", "darwin"} {
		if slices.Contains(launchArgs(goos, false), "--single-process") {
			t.Errorf("unexpected --single-process on %s", goos)
		}
	}
}

func TestLaunchArgsKeepSingleProcessOnLinux(t *testing.T) {
	t.Parallel()

	if !slices.Contains(launchArgs("linux", false), "--single-process") {
		t.Error("expected --single-process on linux")
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
