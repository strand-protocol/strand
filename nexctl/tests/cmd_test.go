package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nexus-protocol/nexus/nexctl/cmd"
	"github.com/nexus-protocol/nexus/nexctl/pkg/api"
	"github.com/nexus-protocol/nexus/nexctl/pkg/output"
)

func setupTest() {
	cmd.SetClient(&api.MockClient{})
	cmd.SetFormatter(output.NewFormatter("table"))
}

func executeCommand(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root := cmd.RootCmd()
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestVersionCommand(t *testing.T) {
	setupTest()
	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	if !strings.Contains(out, "nexctl version") {
		t.Errorf("expected output to contain 'nexctl version', got: %s", out)
	}
	if !strings.Contains(out, "API server:") {
		t.Errorf("expected output to contain 'API server:', got: %s", out)
	}
}

func TestNodeListCommand(t *testing.T) {
	setupTest()
	out, err := executeCommand("node", "list")
	if err != nil {
		t.Fatalf("node list command failed: %v", err)
	}
	if !strings.Contains(out, "node-alpha-01") {
		t.Errorf("expected output to contain 'node-alpha-01', got: %s", out)
	}
	if !strings.Contains(out, "node-beta-02") {
		t.Errorf("expected output to contain 'node-beta-02', got: %s", out)
	}
	if !strings.Contains(out, "node-gamma-03") {
		t.Errorf("expected output to contain 'node-gamma-03', got: %s", out)
	}
}

func TestNodeDescribeCommand(t *testing.T) {
	setupTest()
	out, err := executeCommand("node", "describe", "node-alpha-01")
	if err != nil {
		t.Fatalf("node describe command failed: %v", err)
	}
	if !strings.Contains(out, "node-alpha-01") {
		t.Errorf("expected output to contain 'node-alpha-01', got: %s", out)
	}
}

func TestNodeDescribeNotFound(t *testing.T) {
	setupTest()
	_, err := executeCommand("node", "describe", "nonexistent-node")
	if err == nil {
		t.Fatalf("expected error for nonexistent node, got nil")
	}
}

func TestNodeDrainCommand(t *testing.T) {
	setupTest()
	// --yes skips the interactive confirmation prompt (required in non-TTY test env)
	out, err := executeCommand("node", "drain", "--yes", "node-alpha-01")
	if err != nil {
		t.Fatalf("node drain command failed: %v", err)
	}
	if !strings.Contains(out, "drained successfully") {
		t.Errorf("expected output to contain 'drained successfully', got: %s", out)
	}
}

func TestNodeDrainDryRun(t *testing.T) {
	setupTest()
	out, err := executeCommand("node", "drain", "--dry-run", "node-alpha-01")
	if err != nil {
		t.Fatalf("node drain --dry-run command failed: %v", err)
	}
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected output to contain 'dry-run', got: %s", out)
	}
	// Dry-run must NOT actually drain the node.
	if strings.Contains(out, "drained successfully") {
		t.Errorf("dry-run must not execute drain, got: %s", out)
	}
}

func TestRouteListCommand(t *testing.T) {
	setupTest()
	out, err := executeCommand("route", "list")
	if err != nil {
		t.Fatalf("route list command failed: %v", err)
	}
	if !strings.Contains(out, "sad:llm-inference:gpt4:128k") {
		t.Errorf("expected output to contain SAD entry, got: %s", out)
	}
	if !strings.Contains(out, "sad:embedding:bge-large:8k") {
		t.Errorf("expected output to contain SAD entry, got: %s", out)
	}
}

func TestRouteDescribeCommand(t *testing.T) {
	setupTest()
	out, err := executeCommand("route", "describe", "sad:llm-inference:gpt4:128k")
	if err != nil {
		t.Fatalf("route describe command failed: %v", err)
	}
	if !strings.Contains(out, "sad:llm-inference:gpt4:128k") {
		t.Errorf("expected output to contain SAD, got: %s", out)
	}
}

func TestRouteAddCommand(t *testing.T) {
	setupTest()
	out, err := executeCommand("route", "add", "--model-type", "llm-inference")
	if err != nil {
		t.Fatalf("route add command failed: %v", err)
	}
	if !strings.Contains(out, "added successfully") {
		t.Errorf("expected output to contain 'added successfully', got: %s", out)
	}
}

func TestFirmwareListCommand(t *testing.T) {
	setupTest()
	out, err := executeCommand("firmware", "list")
	if err != nil {
		t.Fatalf("firmware list command failed: %v", err)
	}
	if !strings.Contains(out, "ConnectX-7") {
		t.Errorf("expected output to contain 'ConnectX-7', got: %s", out)
	}
}

func TestFirmwareFlashCommand(t *testing.T) {
	setupTest()
	out, err := executeCommand("firmware", "flash", "node-alpha-01", "nexlink-v2.4.1")
	if err != nil {
		t.Fatalf("firmware flash command failed: %v", err)
	}
	if !strings.Contains(out, "flashed") {
		t.Errorf("expected output to contain 'flashed', got: %s", out)
	}
}

func TestDiagnosePingCommand(t *testing.T) {
	setupTest()
	out, err := executeCommand("diagnose", "ping", "node-alpha-01")
	if err != nil {
		t.Fatalf("diagnose ping command failed: %v", err)
	}
	if !strings.Contains(out, "node-alpha-01") {
		t.Errorf("expected output to contain target node, got: %s", out)
	}
}

func TestMetricsShowCommand(t *testing.T) {
	setupTest()
	out, err := executeCommand("metrics", "show", "--node", "node-alpha-01")
	if err != nil {
		t.Fatalf("metrics show command failed: %v", err)
	}
	if !strings.Contains(out, "node-alpha-01") {
		t.Errorf("expected output to contain node ID, got: %s", out)
	}
}

func TestMetricsExportJSON(t *testing.T) {
	setupTest()
	out, err := executeCommand("metrics", "export", "--format", "json")
	if err != nil {
		t.Fatalf("metrics export json command failed: %v", err)
	}
	if !strings.Contains(out, "node_id") {
		t.Errorf("expected JSON output with node_id, got: %s", out)
	}
}

func TestMetricsExportPrometheus(t *testing.T) {
	setupTest()
	out, err := executeCommand("metrics", "export", "--format", "prometheus")
	if err != nil {
		t.Fatalf("metrics export prometheus command failed: %v", err)
	}
	if !strings.Contains(out, "nexus_connections") {
		t.Errorf("expected Prometheus output with nexus_connections, got: %s", out)
	}
}

func TestTrustIssueMIC(t *testing.T) {
	setupTest()
	out, err := executeCommand("trust", "issue-mic", "--node", "node-alpha-01")
	if err != nil {
		t.Fatalf("trust issue-mic command failed: %v", err)
	}
	if !strings.Contains(out, "node-alpha-01") {
		t.Errorf("expected output to contain node ID, got: %s", out)
	}
}

func TestTrustListCAs(t *testing.T) {
	setupTest()
	out, err := executeCommand("trust", "list-cas")
	if err != nil {
		t.Fatalf("trust list-cas command failed: %v", err)
	}
	if !strings.Contains(out, "nexus-root-ca") {
		t.Errorf("expected output to contain 'nexus-root-ca', got: %s", out)
	}
}

func TestJSONOutputFormat(t *testing.T) {
	cmd.SetClient(&api.MockClient{})
	cmd.SetFormatter(output.NewFormatter("json"))
	out, err := executeCommand("node", "list", "-o", "json")
	if err != nil {
		t.Fatalf("node list json command failed: %v", err)
	}
	if !strings.Contains(out, "\"id\"") {
		t.Errorf("expected JSON output with 'id' field, got: %s", out)
	}
}

func TestYAMLOutputFormat(t *testing.T) {
	cmd.SetClient(&api.MockClient{})
	cmd.SetFormatter(output.NewFormatter("yaml"))
	out, err := executeCommand("node", "list", "-o", "yaml")
	if err != nil {
		t.Fatalf("node list yaml command failed: %v", err)
	}
	if !strings.Contains(out, "id:") {
		t.Errorf("expected YAML output with 'id:' field, got: %s", out)
	}
}
