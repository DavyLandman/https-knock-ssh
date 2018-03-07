#!/usr/bin/env bash

# safer bash script 
set -o nounset -o errexit -o pipefail
# don't split on spaces, only on lines
IFS=$'\n\t'

readonly TARGET="$1"

kill_descendant_processes() {
    local pid="$1"
    local and_self="${2:-false}"
    if children="$(pgrep -P "$pid")"; then
        for child in $children; do
            kill_descendant_processes "$child" true
        done
    fi
    if [[ "$and_self" == true ]]; then
        kill "$pid"
    fi
}

go run "test/server.go" --port 5000 &
readonly SERVER_PID=$!
kill_server() {
    kill $SERVER_PID
}
trap kill_server EXIT

VALGRIND="false"
if [ $# -eq 2 ]; then
    if [[ "$2" == "--valgrind" ]]; then
        VALGRIND="true"
    fi
fi
if [[ "$VALGRIND" == "true" ]]; then
    valgrind --log-file='valgrind.log' -v --leak-check=full --show-leak-kinds=all $TARGET --normalPort=5000 --listenPort=6000 --proxyTimeout=4 --knockTimeout=1 -- HELLO 2> /dev/null &
else
    $TARGET --normalPort=5000 --listenPort=6000 --proxyTimeout=4 --knockTimeout=1 HELLO 2> /dev/null &
fi
readonly PROXY_PID=$!

kill_proxy() {
    kill $PROXY_PID || true
    wait $PROXY_PID || true
    if [[ "$VALGRIND" == "true" ]]; then
        cat 'valgrind.log'
        rm 'valgrind.log' || true
    fi
}
trap kill_proxy ERR

sleep 2

HIDE_PROGRESS=""
if [ ! -z "${CI+x}" ]; then
    if [[ "$CI" == "true" ]]; then
        HIDE_PROGRESS="--hideProgress"
    fi
fi
run_test() {
    go run "test/client.go" --port 6000  --connections "$1" --parallel "$2" $HIDE_PROGRESS
}

echo "" 
echo "/----------------"
echo "| Running single threaded test case"
echo "\\----------------"
run_test 20 1
run_test 200 1

echo "" 
echo "/----------------"
echo "| Running multi-threaded test case"
echo "\\----------------"
run_test 20 20
run_test 200 40

echo "" 
echo "/----------------"
echo "| Running time-out test cases"
echo "\\----------------"
echo " + Within the proxy window"
go run "test/client.go" --port 6000  --connections 5 --parallel 4 --maxDelays 2 $HIDE_PROGRESS
echo " + Sometimes outside the proxy window"
go run "test/client.go" --port 6000  --connections 5 --parallel 4 --maxDelays 5 $HIDE_PROGRESS && rc=$? || rc=$?
if [ $rc -ne 1 ]; then
    echo "The timeouts outside the windows should have failed"
    exit 1
fi

kill $PROXY_PID
wait $PROXY_PID || true

if [[ "$VALGRIND" == "true" ]]; then
    cat 'valgrind.log'
    rm 'valgrind.log' || true
fi

echo "/----------------"
echo "| All test are green"
echo "\\----------------"
