#!/bin/bash
# installing postgres helm chart under a given release name
# add jetstack repository
kubectl apply -f https://raw.githubusercontent.com/pixie-labs/pixie/main/k8s/operator/crd/base/px.dev_viziers.yaml
kubectl apply -f https://raw.githubusercontent.com/pixie-labs/pixie/main/k8s/operator/helm/crds/olm_crd.yaml
helm repo add newrelic https://helm-charts.newrelic.com
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add jetstack https://charts.jetstack.io || true
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo add kube-state-metrics https://kubernetes.github.io/kube-state-metrics
helm repo add podinfo https://stefanprodan.github.io/podinfo
helm repo update

kubectl create namespace newrelic
helm upgrade --install newrelic-bundle newrelic/nri-bundle \
 --set global.licenseKey="NRJS-289e4caff97051d3722" \
 --set global.cluster=development \
 --namespace=newrelic \
 --set newrelic-infrastructure.privileged=true \
 --set ksm.enabled=true \
 --set prometheus.enabled=true \
 --set kubeEvents.enabled=true \
 --set logging.enabled=true \
 --set newrelic-pixie.enabled=true \
 --set newrelic-pixie.apiKey="px-dep-47a828ae-259d-4ce6-a143-3f28ac7fa090" \
 --set pixie-chart.enabled=true \
 --set pixie-chart.deployKey="px-api-96674db8-6d46-482f-92af-f48b416a9032" \
 --set pixie-chart.clusterName=development

helm upgrade --install service ./charts/service

kubectl rollout status deployments/service=5m

helm test service
