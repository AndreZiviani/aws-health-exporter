package main

import "testing"

// TestNewMeterProvider guards against schema URL conflicts between
// resource.Default() and the semconv version imported by this package, which
// only surface at runtime.
func TestNewMeterProvider(t *testing.T) {
	provider, err := newMeterProvider()
	if err != nil {
		t.Fatalf("newMeterProvider() returned error: %v", err)
	}

	if err := provider.Shutdown(t.Context()); err != nil {
		t.Fatalf("Shutdown() returned error: %v", err)
	}
}
