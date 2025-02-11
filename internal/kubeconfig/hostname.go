package kubeconfig

import (
	"fmt"

	"github.com/mproffitt/kubeconfig-operator/internal/helpers"
)

var remapDomains = AllowedDomains{
	"localhost",
	"127.0.0.1",
	"localhost.localdomain",
}

func remap(address, remapAddress string) (string, string, string, error) {
	var (
		scheme, host, port string
		err                error
	)

	if scheme, host, port, err = helpers.AddressToSchemeHostPort(address); err != nil {
		return address, address, port, err
	}

	if remapAddress != "" && remapDomains.Has(host) {
		// localhost and localhost.localdomain are remapped to
		// 127.0.0.1 for creating firewall rules.
		if host == "localhost" || host == "localhost.localdomain" {
			host = "127.0.0.1"
		}
		return fmt.Sprintf("%s://%s:%s", scheme, remapAddress, port), host, port, nil
	}

	return address, host, port, nil
}
