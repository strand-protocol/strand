package api

// APIClient defines the interface for communicating with the Nexus control plane.
type APIClient interface {
	// Node management
	ListNodes() ([]NodeInfo, error)
	DescribeNode(id string) (*NodeInfo, error)
	DrainNode(id string) error

	// Route management
	ListRoutes() ([]RouteInfo, error)
	DescribeRoute(sad string) (*RouteInfo, error)
	AddRoute(r RouteInfo) error

	// Trust / MIC management
	IssueMIC(nodeID string) (*MICInfo, error)
	VerifyMIC(data []byte) (*MICInfo, error)
	ListCAs() ([]string, error)

	// Diagnostics
	Ping(target string) (*DiagnoseResult, error)

	// Metrics
	GetMetrics(nodeID string) (*MetricsData, error)

	// Firmware
	ListFirmware() ([]FirmwareInfo, error)
	FlashFirmware(device, image string) error

	// Version
	Version() (string, error)
}
