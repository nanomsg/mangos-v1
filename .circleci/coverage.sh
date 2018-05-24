#!/bin/sh

covs=
typeset -i num=0
covers=

for pkg in $(go list ./...); do
	echo "Testing ${pkg}"
	cover=cover${num}.txt
	covers="$covers $cover"
	go test -coverpkg=./... -covermode=count -coverprofile=${cover} ${pkg}
	num=$(( num + 1 ))
done

# Merge all test runs.
go get github.com/wadey/gocovmerge
go build github.com/wadey/gocovmerge
${HOME}/go/bin/gocovmerge ${covers} > coverage.txt
