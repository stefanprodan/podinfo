package main

import data.kubernetes

name = input.metadata.name

# Deny containers with latest image tag
deny[msg] {
	kubernetes.containers[container]
	[image_name, "latest"] = kubernetes.split_image(container.image)
	msg = sprintf("%s in the %s %s has an image %s, using the latest tag", [container.name, kubernetes.kind, kubernetes.name, image_name])
}

# Deny services without app label selector
service_labels {
  input.spec.selector["app"]
}
deny[msg] {
  kubernetes.is_service
  not service_labels
  msg = sprintf("Service %s should set app label selector", [name])
}

# Deny deployments without app label selector
match_labels {
  input.spec.selector.matchLabels["app"]
}
deny[msg] {
  kubernetes.is_deployment
  not match_labels
  msg = sprintf("Service %s should set app label selector", [name])
}

# Warn if deployments have no prometheus pod annotations
annotations {
  input.spec.template.metadata.annotations["prometheus.io/scrape"]
  input.spec.template.metadata.annotations["prometheus.io/port"]
}
warn[msg] {
  kubernetes.is_deployment
  not annotations
  msg = sprintf("Deployment %s should set prometheus.io/scrape and prometheus.io/port pod annotations", [name])
}
