module github.com/nexus-protocol/nexus/tests/bench

go 1.22

require (
	github.com/nexus-protocol/nexus/nexapi v0.0.0
)

replace (
	github.com/nexus-protocol/nexus/nexapi => ../../nexapi
)
