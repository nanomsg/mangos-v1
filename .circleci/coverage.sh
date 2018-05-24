#!/bin/sh

covs=
typeset -i num=0
covers=

scratch=$(mktemp -d)
trap "rm -rf $scratch" 0

template=${scratch}/covXXXXXX
pkgs=github.com/go-mangos/mangos/...

find . -type d -print | while read dir; do
	(
	cd $dir
	if compgen -G "*_test.go" > /dev/null; then
		echo "Doing test in $dir"
		out=$(mktemp -u $template)
		go test -coverpkg=${pkgs} -covermode=count -coverprofile=${out} .
		mv ${out} ${out}.out
	fi
	)
done

#for pkg in $(go list ./...); do
#	echo "Testing ${pkg}"
#	cover=cover${num}.txt
#	covers="$covers $cover"
#	go test -coverpkg=./... -covermode=count -coverprofile=${cover} ${pkg}
#	num=$(( num + 1 ))
#done

# Merge all test runs.
go get github.com/wadey/gocovmerge
go build github.com/wadey/gocovmerge
#${HOME}/go/bin/gocovmerge ${covers} > coverage.txt
${HOME}/go/bin/gocovmerge ${scratch}/*.out > coverage.txt
