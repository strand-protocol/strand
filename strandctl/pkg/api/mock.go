package api

import (
	"fmt"
	"time"
)

// MockClient implements APIClient with canned data for development and testing.
type MockClient struct{}

var _ APIClient = (*MockClient)(nil)

func (m *MockClient) ListNodes() ([]NodeInfo, error) {
	return []NodeInfo{
		{
			ID:           "node-alpha-01",
			Address:      "10.0.1.10:9100",
			Status:       "ready",
			LastSeen:     time.Now().Add(-30 * time.Second),
			Firmware:     "nexlink-v2.4.1",
			Capabilities: []string{"llm-inference", "embedding", "rerank"},
		},
		{
			ID:           "node-beta-02",
			Address:      "10.0.1.11:9100",
			Status:       "ready",
			LastSeen:     time.Now().Add(-15 * time.Second),
			Firmware:     "nexlink-v2.4.1",
			Capabilities: []string{"llm-inference", "vision"},
		},
		{
			ID:           "node-gamma-03",
			Address:      "10.0.1.12:9100",
			Status:       "draining",
			LastSeen:     time.Now().Add(-120 * time.Second),
			Firmware:     "nexlink-v2.3.0",
			Capabilities: []string{"embedding"},
		},
	}, nil
}

func (m *MockClient) DescribeNode(id string) (*NodeInfo, error) {
	nodes, _ := m.ListNodes()
	for _, n := range nodes {
		if n.ID == id {
			return &n, nil
		}
	}
	return nil, fmt.Errorf("node %q not found", id)
}

func (m *MockClient) DrainNode(id string) error {
	_, err := m.DescribeNode(id)
	if err != nil {
		return err
	}
	return nil
}

func (m *MockClient) ListRoutes() ([]RouteInfo, error) {
	return []RouteInfo{
		{
			SAD:       "sad:llm-inference:gpt4:128k",
			Endpoints: []string{"node-alpha-01", "node-beta-02"},
			Weight:    100,
			TTL:       300,
		},
		{
			SAD:       "sad:embedding:bge-large:8k",
			Endpoints: []string{"node-alpha-01", "node-gamma-03"},
			Weight:    80,
			TTL:       600,
		},
		{
			SAD:       "sad:vision:llava:4k",
			Endpoints: []string{"node-beta-02"},
			Weight:    50,
			TTL:       300,
		},
	}, nil
}

func (m *MockClient) DescribeRoute(sad string) (*RouteInfo, error) {
	routes, _ := m.ListRoutes()
	for _, r := range routes {
		if r.SAD == sad {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("route %q not found", sad)
}

func (m *MockClient) AddRoute(r RouteInfo) error {
	if r.SAD == "" {
		return fmt.Errorf("SAD is required")
	}
	return nil
}

func (m *MockClient) IssueMIC(nodeID string) (*MICInfo, error) {
	return &MICInfo{
		NodeID:     nodeID,
		ModelHash:  "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
		ValidUntil: time.Now().Add(365 * 24 * time.Hour),
		Issuer:     "nexus-root-ca",
		Status:     "valid",
	}, nil
}

func (m *MockClient) VerifyMIC(data []byte) (*MICInfo, error) {
	return &MICInfo{
		NodeID:     "node-alpha-01",
		ModelHash:  "sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
		ValidUntil: time.Now().Add(180 * 24 * time.Hour),
		Issuer:     "nexus-root-ca",
		Status:     "valid",
	}, nil
}

func (m *MockClient) ListCAs() ([]string, error) {
	return []string{
		"nexus-root-ca",
		"nexus-intermediate-ca-1",
		"nexus-intermediate-ca-2",
	}, nil
}

func (m *MockClient) Ping(target string) (*DiagnoseResult, error) {
	return &DiagnoseResult{
		Target:  target,
		Latency: 1.23,
		Hops:    3,
		Status:  "ok",
	}, nil
}

func (m *MockClient) GetMetrics(nodeID string) (*MetricsData, error) {
	return &MetricsData{
		NodeID:      nodeID,
		Connections: 42,
		BytesSent:   1073741824,
		BytesRecv:   2147483648,
		Latency:     0.85,
	}, nil
}

func (m *MockClient) ListFirmware() ([]FirmwareInfo, error) {
	return []FirmwareInfo{
		{
			ID:       "fw-001",
			Version:  "nexlink-v2.4.1",
			Platform: "ConnectX-7",
			Size:     16777216,
			Checksum: "sha256:deadbeef01234567",
		},
		{
			ID:       "fw-002",
			Version:  "nexlink-v2.4.1",
			Platform: "E810",
			Size:     12582912,
			Checksum: "sha256:cafebabe89abcdef",
		},
		{
			ID:       "fw-003",
			Version:  "nexlink-v2.3.0",
			Platform: "BlueField-3",
			Size:     33554432,
			Checksum: "sha256:0123456789abcdef",
		},
	}, nil
}

func (m *MockClient) FlashFirmware(device, image string) error {
	if device == "" || image == "" {
		return fmt.Errorf("device and image are required")
	}
	return nil
}

func (m *MockClient) Version() (string, error) {
	return "nexus-api v0.1.0 (mock)", nil
}
