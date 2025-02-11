package kubeconfig

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd/api"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func ClientConfig(context, remapAddress string, options GetContextsOptions) (*kconfig, error) {
	c := &kconfig{}
	var err error

	if c.Config, err = clientconfig.GetConfigWithContext(context); err != nil {
		return nil, errors.Wrap(err, "cannot get cluster config with context "+context)
	}

	config, err := options.configAccess.GetStartingConfig()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get starting config")
	}

	var (
		user     string
		ctx      *api.Context
		cluster  *api.Cluster
		ok       bool
		authInfo *api.AuthInfo
	)
	if ctx, ok = config.Contexts[context]; !ok {
		return nil, errors.New("context not found")
	}

	if cluster, ok = config.Clusters[ctx.Cluster]; !ok {
		return nil, errors.New("cluster not found")
	}

	user = ctx.AuthInfo
	if authInfo, ok = config.AuthInfos[user]; !ok {
		return nil, errors.New("auth info not found")
	}

	var cmd string
	if authInfo.Exec != nil {
		cmd = authInfo.Exec.Command
	}

	switch cmd {
	case "aws-iam-authenticator", "aws":
		c.provider = ProviderKindAWS
	default:
		c.provider = ProviderKindClientCert
	}

	host, oldhost, port, err := remap(cluster.Server, remapAddress)
	if err != nil {
		return nil, errors.Wrap(err, "cannot remap host")
	}

	c.originalIp = oldhost
	c.remappedIp = remapAddress
	c.port = port

	c.Host = host
	c.CAData = cluster.CertificateAuthorityData
	c.CAFile = cluster.CertificateAuthority

	c.CertData = authInfo.ClientCertificateData
	c.KeyData = authInfo.ClientKeyData
	c.Username = authInfo.Username
	if c.Username == "" {
		c.Username = user
	}

	return c, nil
}
