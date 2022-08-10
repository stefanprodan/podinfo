#!/bin/bash 

namespace=staging
while getopts n: flag
do
    case "${flag}" in
        n) namespace=${OPTARG};;
    esac
done

namespaceStatus=$(kubectl get ns ${namespace} -o json | jq .status.phase -r)
if [[ $namespaceStatus != "Active" ]]; then 
    echo "creating namespace ($namespace) in which all services will be deployed"
    kubectl create namespace ${namespace}
fi

echo "installing service in default namespace"
helm upgrade --install service ./charts/service --values ./charts/service/values.production.yaml -n ${namespace}