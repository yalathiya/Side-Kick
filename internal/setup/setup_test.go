package setup

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSetup_CoreOnly(t *testing.T) {
	// Simulate user pressing enter for all defaults, "n" for all optional features
	input := strings.NewReader("\n\n\n\nn\nn\nn\nn\n")
	output := &bytes.Buffer{}

	r := &Runner{In: input, Out: output}
	err := r.Run()
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer os.Remove(".env")

	data, err := os.ReadFile(".env")
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "SIDEKICK_PORT=8081") {
		t.Error("expected default port 8081 in .env")
	}
	if !strings.Contains(content, "SIDEKICK_UPSTREAM_URL=http://localhost:8080") {
		t.Error("expected default upstream URL in .env")
	}
	// Phase 2 vars should NOT be present
	if strings.Contains(content, "SIDEKICK_JWT_SECRET") {
		t.Error("JWT config should not be in .env when skipped")
	}
	if strings.Contains(content, "SIDEKICK_REDIS_URL") {
		t.Error("Redis config should not be in .env when skipped")
	}
}

func TestSetup_WithJWT(t *testing.T) {
	// Accept defaults for core, enable JWT with custom secret, skip rest
	input := strings.NewReader("\n\n\n\ny\nmy-secret\n\n\nn\nn\nn\n")
	output := &bytes.Buffer{}

	r := &Runner{In: input, Out: output}
	err := r.Run()
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer os.Remove(".env")

	data, _ := os.ReadFile(".env")
	content := string(data)
	if !strings.Contains(content, "SIDEKICK_JWT_SECRET=my-secret") {
		t.Error("expected JWT secret in .env")
	}
}

func TestSetup_CustomPort(t *testing.T) {
	input := strings.NewReader("9090\n\n\n\nn\nn\nn\nn\n")
	output := &bytes.Buffer{}

	r := &Runner{In: input, Out: output}
	err := r.Run()
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer os.Remove(".env")

	data, _ := os.ReadFile(".env")
	if !strings.Contains(string(data), "SIDEKICK_PORT=9090") {
		t.Error("expected custom port 9090 in .env")
	}
}

func TestSetup_OutputContainsBanner(t *testing.T) {
	input := strings.NewReader("\n\n\n\nn\nn\nn\nn\n")
	output := &bytes.Buffer{}

	r := &Runner{In: input, Out: output}
	r.Run()
	defer os.Remove(".env")

	if !strings.Contains(output.String(), "Sidekick") {
		t.Error("expected setup banner in output")
	}
	if !strings.Contains(output.String(), "Config saved") {
		t.Error("expected success message in output")
	}
}
