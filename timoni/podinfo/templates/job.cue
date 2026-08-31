package templates

import (
	"encoding/yaml"
	"uuid"

	corev1 "k8s.io/api/core/v1"
	batchv1 "k8s.io/api/batch/v1"
	timoniv1 "timoni.sh/core/v1alpha1"
)

// The test Job verifying that the podinfo http Service port is
// reachable from inside the cluster. The Job is recreated by Timoni
// when the module version or values change.
#TestJob: batchv1.#Job & {
	_config: #Config
	let cfg = _config

	apiVersion: "batch/v1"
	kind:       "Job"
	metadata: timoniv1.#MetaComponent & {
		#Meta:      cfg.metadata
		#Component: "test"
	}
	metadata: annotations: timoniv1.Action.Force
	spec: batchv1.#JobSpec & {
		template: corev1.#PodTemplateSpec & {
			let _checksum = uuid.SHA1(uuid.ns.DNS, yaml.Marshal(cfg))
			metadata: annotations: "timoni.sh/checksum": "\(_checksum)"
			spec: #TestPodSpec & {
				#config: cfg
				containers: [{
					name:            "curl"
					image:           cfg.test.image.reference
					imagePullPolicy: cfg.test.image.pullPolicy
					securityContext: corev1.#SecurityContext & timoniv1.#ContainerSecurityContext
					command: [
						"curl",
						"-v",
						"-m",
						"5",
						"\(cfg.metadata.name):\(cfg.service.port)",
					]
				}]
			}
		}
		backoffLimit: 1
	}
}

// The test Job verifying the podinfo gRPC health service through
// the grpc Service port, using grpc-health-probe.
#TestGRPCJob: batchv1.#Job & {
	_config: #Config
	let cfg = _config

	apiVersion: "batch/v1"
	kind:       "Job"
	metadata: timoniv1.#MetaComponent & {
		#Meta:      cfg.metadata
		#Component: "test-grpc"
	}
	metadata: annotations: timoniv1.Action.Force
	spec: batchv1.#JobSpec & {
		template: corev1.#PodTemplateSpec & {
			let _checksum = uuid.SHA1(uuid.ns.DNS, yaml.Marshal(cfg))
			metadata: annotations: "timoni.sh/checksum": "\(_checksum)"
			spec: #TestPodSpec & {
				#config: cfg
				containers: [{
					name:            "grpc-health-probe"
					image:           cfg.test.grpcImage.reference
					imagePullPolicy: cfg.test.grpcImage.pullPolicy
					securityContext: corev1.#SecurityContext & timoniv1.#ContainerSecurityContext
					args: [
						"-addr=\(cfg.metadata.name):\(cfg.service.grpcPort)",
						"-connect-timeout=5s",
						"-rpc-timeout=5s",
					]
				}]
			}
		}
		backoffLimit: 1
	}
}

// The pod spec shared by the test Jobs: the pods run once with the
// hardened pod security context and follow the instance placement
// settings (node selector, node affinity, tolerations, pull secrets).
#TestPodSpec: corev1.#PodSpec & {
	#config: #Config

	restartPolicy:   "Never"
	securityContext: #config.podSecurityContext
	nodeSelector:    #config.nodeSelector
	if #config.affinity.nodeAffinity != _|_ {
		affinity: nodeAffinity: #config.affinity.nodeAffinity
	}
	if #config.tolerations != _|_ {
		tolerations: #config.tolerations
	}
	if #config.imagePullSecrets != _|_ {
		imagePullSecrets: #config.imagePullSecrets
	}
}
