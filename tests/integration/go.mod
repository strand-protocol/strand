module github.com/nexus-protocol/nexus/tests/integration

go 1.22

require (
	github.com/nexus-protocol/nexus/nexapi v0.0.0
	github.com/nexus-protocol/nexus/nexus-cloud v0.0.0
)

replace (
	github.com/nexus-protocol/nexus/nexapi => ../../nexapi
	github.com/nexus-protocol/nexus/nexus-cloud => ../../nexus-cloud
)
