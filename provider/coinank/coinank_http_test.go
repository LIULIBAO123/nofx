package coinank

import (
	"os"
	"testing"
)

var TestApikey = "" //need fill the apikey before test

// These tests call external Coinank APIs. Skip when API key is not configured.
// (If you want to run them locally, set TestApikey above or use an env-driven key and update tests.)
func TestMain(m *testing.M) {
	if TestApikey == "" {
		os.Exit(0)
	}
	os.Exit(m.Run())
}
