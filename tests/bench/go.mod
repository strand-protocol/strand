module github.com/strand-protocol/strand/tests/bench

go 1.22

require (
	github.com/strand-protocol/strand/strandapi v0.0.0
)

replace (
	github.com/strand-protocol/strand/strandapi => ../../strandapi
)
