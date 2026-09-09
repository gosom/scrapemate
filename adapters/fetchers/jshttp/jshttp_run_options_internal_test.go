package jshttp

import (
	"os"
	"testing"
)

func TestPlaywrightRunOptions(t *testing.T) {
	originalValue, wasSet := os.LookupEnv(skipPlaywrightBrowserInstallEnv)
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv(skipPlaywrightBrowserInstallEnv, originalValue)

			return
		}

		_ = os.Unsetenv(skipPlaywrightBrowserInstallEnv)
	})

	testCases := []struct {
		name                string
		value               string
		unset               bool
		skipInstallBrowsers bool
	}{
		{name: "unset", unset: true},
		{name: "empty"},
		{name: "zero", value: "0"},
		{name: "arbitrary", value: "true"},
		{name: "enabled", value: "1", skipInstallBrowsers: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var err error
			if testCase.unset {
				err = os.Unsetenv(skipPlaywrightBrowserInstallEnv)
			} else {
				err = os.Setenv(skipPlaywrightBrowserInstallEnv, testCase.value)
			}
			if err != nil {
				t.Fatalf("set test environment: %v", err)
			}

			options := playwrightRunOptions()
			if len(options) != 1 {
				t.Fatalf("expected one Playwright run option, got %d", len(options))
			}
			if options[0] == nil {
				t.Fatal("expected non-nil Playwright run option")
			}
			if len(options[0].Browsers) != 1 || options[0].Browsers[0] != "chromium" {
				t.Fatalf("expected only Chromium, got %v", options[0].Browsers)
			}
			if !options[0].Verbose {
				t.Fatal("expected verbose Playwright installation")
			}
			if options[0].SkipInstallBrowsers != testCase.skipInstallBrowsers {
				t.Fatalf(
					"expected SkipInstallBrowsers=%t, got %t",
					testCase.skipInstallBrowsers,
					options[0].SkipInstallBrowsers,
				)
			}
		})
	}
}
