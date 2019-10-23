package kubernetes

name = input.metadata.name

kind = input.kind

is_service {
	input.kind = "Service"
}

is_deployment {
	input.kind = "Deployment"
}

is_pod {
	input.kind = "Pod"
}

split_image(image) = [image, "latest"] {
	not contains(image, ":")
}

split_image(image) = [image_name, tag] {
	[image_name, tag] = split(image, ":")
}

pod_containers(pod) = all_containers {
	keys = {"containers", "initContainers"}
	all_containers = [c | keys[k]; c = pod.spec[k][_]]
}

containers[container] {
	pods[pod]
	all_containers = pod_containers(pod)
	container = all_containers[_]
}

containers[container] {
	all_containers = pod_containers(input)
	container = all_containers[_]
}

pods[pod] {
	is_deployment
	pod = input.spec.template
}

pods[pod] {
	is_pod
	pod = input
}
