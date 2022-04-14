package podinfo

import (
	certmanv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"encoding/yaml"
)

#certConfig: {
	dnsNames: [string]
	tlsSecretName: string
	issuerRef:     string
}

#Certificate: certmanv1.#Certificate & {
	_config:    #Config
	apiVersion: "v1"
	kind:       "Certificate"
	metadata:   _config.meta
	spec:       certmanv1.#CertificateSpec & {
		dnsNames:   _config.cert.dnsNames
		secretName: _config.cert.tlsSecretName
		issuerRef:  yaml.Marshal(_config.cert.issuerRef)
	}
}
