package kubeconfig

import (
	"github.com/mproffitt/kubeconfig-operator/internal/helpers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type ProviderKind string

const (
	ProviderKindUnknown    ProviderKind = "unknown"
	ProviderKindAWS        ProviderKind = "aws"
	ProviderKindAzure      ProviderKind = "azure"
	ProviderKindGCP        ProviderKind = "gcp"
	ProviderKindOIDC       ProviderKind = "oidc"
	ProviderKindClientCert ProviderKind = "client-cert"
)

var DefaultAllowedDomains = AllowedDomains{
	"cluster.local",
	"localhost",
	"127.0.0.1",
	"localhost.localdomain",
	"localhost.localstack.cloud",
}

// AllowedDomain is a domain that is allowed to be used in a kubeconfig.
type AllowedDomain string

// AllowedDomains is a list of allowed domains.
type AllowedDomains []AllowedDomain

// NewAllowedDomains creates a new AllowedDomains from a list of domains.
func NewAllowedDomains(domains []string) AllowedDomains {
	var size int = len(domains) + len(DefaultAllowedDomains)
	var allowedDomains = make(AllowedDomains, 0, size)
	allowedDomains = append(allowedDomains, DefaultAllowedDomains...)
	for _, domain := range domains {
		allowedDomains = append(allowedDomains, AllowedDomain(domain))
	}

	return allowedDomains
}

// Has checks if a domain is in the list of allowed domains.
func (a AllowedDomains) Has(domain string) bool {
	for _, d := range a {
		if string(d) == domain {
			return true
		}
		_, host, _, err := helpers.AddressToSchemeHostPort(domain)
		if err != nil {
			continue
		}

		if string(d) == host {
			return true
		}
	}

	return false
}

// GetContextsOptions is the options for getting contexts.
type GetContextsOptions struct {
	configAccess clientcmd.ConfigAccess
}

// Context is a reference to a context in a kubeconfig.
type Context struct {
	name    string
	user    string
	cluster string
}

// kconfig is a struct that holds the provider type and the rest config.
type kconfig struct {
	provider   ProviderKind
	originalIp string
	remappedIp string
	port       string
	*rest.Config
}

type ContextList []Context

func (c ContextList) Find(name string) (*Context, bool) {
	var context *Context
	for _, ctx := range c {
		if ctx.name == name {
			context = &ctx
			break
		}
	}

	return context, context != nil
}
