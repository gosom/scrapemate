package jshttp

import (
	"bytes"
	"fmt"
	"os"
	"runtime/pprof"
	"testing"
)

const checkGoroutineLeaksEnv = "SCRAPEMATE_CHECK_GOROUTINE_LEAKS"

func TestMain(m *testing.M) {
	exitCode := m.Run()
	if exitCode != 0 || os.Getenv(checkGoroutineLeaksEnv) != "1" {
		os.Exit(exitCode)
	}

	profile := pprof.Lookup("goroutineleak")
	if profile == nil {
		fmt.Fprintln(os.Stderr, "goroutineleak profile is unavailable")
		os.Exit(1)
	}

	var report bytes.Buffer
	if err := profile.WriteTo(&report, 1); err != nil {
		fmt.Fprintf(os.Stderr, "write goroutineleak profile: %v\n", err)
		os.Exit(1)
	}

	if profile.Count() > 0 {
		fmt.Fprintln(os.Stderr, "leaked goroutines detected:")
		_, _ = report.WriteTo(os.Stderr)

		os.Exit(1)
	}

	os.Exit(0)
}
