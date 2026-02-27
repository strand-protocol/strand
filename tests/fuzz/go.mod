module github.com/strand-protocol/strand/tests/fuzz

go 1.22

require (
	github.com/strand-protocol/strand/strandapi v0.0.0
)

replace (
	github.com/strand-protocol/strand/strandapi => ../../strandapi
)
