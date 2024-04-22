#!/bin/bash

# Sourced from scenario.sh and uses functions defined there.

# Override the default test timeout of 30m
# shellcheck disable=SC2034  # used elsewhere
TEST_EXECUTION_TIMEOUT="1.5h"

scenario_create_vms() {
    prepare_kickstart host1 kickstart-bootc.ks.template rhel94-bootc-source
    # Using centos9 is necessary for getting the latest anaconda.
    # It is a temporary workaround until rhel-9.4.iso build is available.
    launch_vm host1 centos9 "" "" "" "" "" "" "1"
}

scenario_remove_vms() {
    remove_vm host1
}

scenario_run_tests() {
    run_tests host1 suites/osconfig
}