package clientcert

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

func KubeConfig(context string, config *rest.Config) (*api.Config, error) {
	cfg := &api.Config{
		APIVersion: api.SchemeGroupVersion.Version,
		Clusters: map[string]*api.Cluster{
			context: {
				Server:                   config.Host,
				CertificateAuthorityData: config.CAData,
			},
		},
		Contexts: map[string]*api.Context{
			context: {
				Cluster:  context,
				AuthInfo: config.Username,
			},
		},
		CurrentContext: context,
		AuthInfos: map[string]*api.AuthInfo{
			config.Username: {
				ClientKeyData:         config.KeyData,
				ClientCertificateData: config.CertData,
			},
		},
	}

	return cfg, nil
}
