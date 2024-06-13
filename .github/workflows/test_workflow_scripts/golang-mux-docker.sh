#!/bin/bash

source ./../../.github/workflows/test_workflow_scripts/test-iid.sh

# Start mongo before starting keploy.
docker network create keploy-network
docker run -p 3306:3306 --rm --name mysql --net keploy-network -e MYSQL_ROOT_PASSWORD=my-secret-pw -d mysql:latest

# Remove any preexisting keploy tests and mocks.
sudo rm -rf keploy/
# Start keploy in record mode.
docker build -t url-short .

container_kill() {
    pid=$(pgrep -n keploy)
    echo "$pid Keploy PID" 
    echo "Killing keploy"
    sudo kill $pid
}

send_request() {
    sleep 10
    app_started=false
    while [ "$app_started" = false ]; do
        if curl -X GET http://localhost:8080/all; then
            app_started=true
        fi
        sleep 3
    done
    echo "App started"
    curl -X POST http://localhost:8080/create \
    -H "Content-Type: application/json" \
    -d '{"link":"https://google.com"}'

    curl http://localhost:8080/links/1

    curl http://localhost:8080/all
    # Wait for 10 seconds for keploy to record the tcs and mocks.
    sleep 10
    pid=$(pgrep keploy)
    echo "$pid Keploy PID"
    echo "Killing keploy"
    sudo kill $pid
}


for i in {1..2}; do
    container_name="urlShort_${i}"
    send_request &
    sudo -E env PATH=$PATH ./../../keployv2 record -c "docker run -p8080:8080 --net keploy-network --rm --name $container_name url-short" --containerName "$container_name" --generateGithubActions=false &> "${container_name}.txt"

    if grep "WARNING: DATA RACE" "${container_name}.txt"; then
        echo "Race condition detected in recording, stopping pipeline..."
        cat "${container_name}.txt"
        exit 1
    fi
    if grep "ERROR" "${container_name}.txt"; then
        echo "Error found in pipeline..."
        cat "${container_name}.txt"
        exit 1
    fi
    sleep 5

    echo "Recorded test case and mocks for iteration ${i}"
done

# Start the keploy in test mode.
test_container="urlShort_test"
sudo -E env PATH=$PATH ./../../keployv2 test -c 'docker run -p8080:8080 --net keploy-network --name ginApp_test url-short' --containerName "$test_container" --apiTimeout 60 --delay 20 --generateGithubActions=false &> "${test_container}.txt"

if grep "ERROR" "${test_container}.txt"; then
    echo "Error found in pipeline..."
    cat "${test_container}.txt"
    exit 1
fi

if grep "WARNING: DATA RACE" "${test_container}.txt"; then
    echo "Race condition detected in test, stopping pipeline..."
    cat "${test_container}.txt"
    exit 1
fi

all_passed=true

for i in {0..1}
do
    # Define the report file for each test set
    report_file="./keploy/reports/test-run-0/test-set-$i-report.yaml"

    # Extract the test status
    test_status=$(grep 'status:' "$report_file" | head -n 1 | awk '{print $2}')

    # Print the status for debugging
    echo "Test status for test-set-$i: $test_status"

    # Check if any test set did not pass
    if [ "$test_status" != "PASSED" ]; then
        all_passed=false
        echo "Test-set-$i did not pass."
        break # Exit the loop early as all tests need to pass
    fi
done

# Check the overall test status and exit accordingly
if [ "$all_passed" = true ]; then
    echo "All tests passed"
    exit 0
else
    cat "${test_container}.txt"
    exit 1
fi