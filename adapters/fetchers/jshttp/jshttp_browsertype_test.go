package jshttp //nolint:testpackage // Need access to unexported browser selection helpers.

import "testing"

func TestBrowsersToInstall(t *testing.T) {
	cases := map[string]string{
		"":         "chromium",
		"chromium": "chromium",
		"firefox":  "firefox",
		"webkit":   "webkit",
		"unknown":  "chromium", // unknown values fall back to chromium
	}

	for in, want := range cases {
		got := browsersToInstall(in)
		if len(got) != 1 || got[0] != want {
			t.Errorf("browsersToInstall(%q) = %v; want [%q]", in, got, want)
		}
	}
}

func TestChromiumLaunchArgs_DisableImages(t *testing.T) {
	const imgFlag = "--blink-settings=imagesEnabled=false"

	with := chromiumLaunchArgs(true)
	if !contains(with, imgFlag) {
		t.Errorf("chromiumLaunchArgs(true) missing %q", imgFlag)
	}

	without := chromiumLaunchArgs(false)
	if contains(without, imgFlag) {
		t.Errorf("chromiumLaunchArgs(false) unexpectedly contains %q", imgFlag)
	}

	// The core Chromium flags must always be present.
	if !contains(without, "--no-sandbox") {
		t.Error("chromiumLaunchArgs missing --no-sandbox")
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}

	return false
}
