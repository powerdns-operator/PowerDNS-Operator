#!/bin/bash

PROJECT_ROOT=$PWD
E2E_DIR=$(realpath $(dirname $0)/..)
CONTROLLER_NAMESPACE=powerdns-operator-system
IMAGE_TAG=`date "+%Y-%m-%d-%H-%M-%S"`
#VERSION=$(cat VERSION | tr -d " \t\n\r")

function build_ginkgo_test() {
  cd $E2E_DIR
  ginkgo build -r e2e/
}

function cleanup() {
  cd $PROJECT_ROOT
#  kubectl delete -f manifests/setup/setup.yaml
#  kubectl delete ns $CONTROLLER_NAMESPACE
  kind delete cluster --name test && exit 0
}

function prepare_cluster() {
  kind create cluster --name test 
  kubectl create ns $CONTROLLER_NAMESPACE

  echo "wait the control-plane ready..."
  kubectl wait --for=condition=Ready node/test-control-plane --timeout=60s
}

function build_image() { 
  cd $PROJECT_ROOT
  make docker-build -e IMG=pdns/powerdns-operator:$IMAGE_TAG
  kind load docker-image pdns/powerdns-operator:$IMAGE_TAG --name test
}

function start_powerdns_operator() {
  cd $PROJECT_ROOT
  make deploy -e IMG=pdns/powerdns-operator:$IMAGE_TAG
  kubectl -n $CONTROLLER_NAMESPACE wait --for=condition=available deployment/powerdns-operator-controller-manager --timeout=60s
}

function deploy_powerdns(){
  kubectl create secret generic powerdns-operator-manager -n $CONTROLLER_NAMESPACE --from-literal=PDNS_API_URL=http://powerdns:8081  --from-literal=PDNS_API_KEY=sPowerDNSAPI --from-literal=PDNS_API_VHOST=localhost
  helm install -n $CONTROLLER_NAMESPACE --version 0.1.3 --set mariadb.primary.persistence.enabled=false --set phpmyadmin.enabled=false --set powerdns-admin.enabled=false powerdns fsdrw08/powerdns
  kubectl -n $CONTROLLER_NAMESPACE wait --for=condition=available deployment/powerdns --timeout=600s
}

function run_test() {
  # inspired by github.com/kubeedge/kubeedge/tests/e2e/scripts/helm_keadm_e2e.sh
  :> /tmp/testcase.log
  $E2E_DIR/e2e/e2e.test $debugflag 2>&1 | tee -a /tmp/testcase.log
  
  grep  -e "Running Suite" -e "SUCCESS\!" -e "FAIL\!" /tmp/testcase.log | sed -r 's/\x1B\[([0-9];)?([0-9]{1,2}(;[0-9]{1,2})?)?[mGK]//g' | sed -r 's/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[mGK]//g'
  echo "Integration Test Final Summary Report"
  echo "======================================================="
  echo "Total Number of Test cases = `grep "Ran " /tmp/testcase.log | awk '{sum+=$2} END {print sum}'`"
  passed=`grep -e "SUCCESS\!" -e "FAIL\!" /tmp/testcase.log | awk '{print $3}' | sed -r "s/\x1B\[([0-9];)?([0-9]{1,2}(;[0-9]{1,2})?)?[mGK]//g" | awk '{sum+=$1} END {print sum}'`
  echo "Number of Test cases PASSED = $passed"
  fail=`grep -e "SUCCESS\!" -e "FAIL\!" /tmp/testcase.log | awk '{print $6}' | sed -r "s/\x1B\[([0-9]{1,2}(;[0-9]{1,2})?)?[mGK]//g" | awk '{sum+=$1} END {print sum}'`
  echo "Number of Test cases FAILED = $fail"
  echo "==================Result Summary======================="

  if [ "$fail" != "0" ];then
      echo "Integration suite has failures, Please check !!"
      exit 1
  else
      echo "Integration suite successfully passed all the tests !!"
      exit 0
  fi
}

set -Ee
trap cleanup EXIT
trap cleanup ERR

echo -e "\nBuilding testcases..."
build_ginkgo_test

echo -e "\nPreparing cluster..."
prepare_cluster

echo -e "\Deploying PowerDNS..."
deploy_powerdns

echo -e "\nBuilding image..."
build_image

echo -e "\nStart powerdns operator..."
start_powerdns_operator

echo -e "\nRunning test..."
run_test

read -t 120 -p "I am going to wait for 120 seconds only ..."
