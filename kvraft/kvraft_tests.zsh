#!/bin/zsh

test_commands=(

    #--------------Raft Tests---------------#

    # "go test -race -run ReElection"
    # "go test -race -run TestBasicAgree"
    # "go test -race -run TestFailAgree"
    # "go test -race -run TestFailNoAgree"
    # "go test -race -run TestConcurrentStarts"
    # "go test -race -run TestRejoin"
    # "go test -race -run TestBackup"
    # "go test -race -run TestPersist1"
    # "go test -race -run TestPersist2"
    # "go test -race -run TestPersist3"
    # "go test -race -run TestCount"
    # "go test -race -run ^TestFigure8$"

    #--------------KvRaft Tests---------------#

    "go test -race -run TestBasic"
    "go test -race -run TestConcurrent"
    "go test -run TestUnreliable"
)

repeat_count=20

typeset -A failure_counts
for cmd in "${test_commands[@]}"; do
    failure_counts[$cmd]=0
done

for cmd in "${test_commands[@]}"; do
    echo "Running tests for: $cmd"
    for i in {1..$repeat_count}; do
        echo "Run #$i for $cmd"
        
        if ! eval "$cmd"; then
            echo "Failure detected in: $cmd"
            ((failure_counts[$cmd]++))
        fi
    done
done


echo "\nTest Failure Summary:"
for cmd in "${test_commands[@]}"; do
    echo "$cmd: ${failure_counts[$cmd]} failures out of $repeat_count runs"
done
