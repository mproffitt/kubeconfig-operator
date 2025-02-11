/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package kubeconfig

import (
	"context"
	"net"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	kccnv1alpha1 "github.com/mproffitt/kubeconfig-operator/api/v1alpha1"
	"github.com/mproffitt/kubeconfig-operator/internal/kubeconfig/aws"
	"github.com/mproffitt/kubeconfig-operator/internal/kubeconfig/clientcert"
	"github.com/pkg/errors"
)

type Manager struct {
	client  client.Client
	context context.Context
	cluster *kccnv1alpha1.Cluster
	log     logr.Logger
}

type Status struct {
	ClusterStatus kccnv1alpha1.ClusterStatusEntries
	FirewallRules []string
	DeletionRules []string
}

func NewManager(
	ctx context.Context, r client.Client, cluster *kccnv1alpha1.Cluster,
) *Manager {
	return &Manager{
		client:  r,
		context: ctx,
		cluster: cluster,
		log:     log.FromContext(ctx),
	}
}

func (m *Manager) ReconcileKubeconfig() (*Status, error) {
	status := &Status{
		ClusterStatus: kccnv1alpha1.ClusterStatusEntries{},
		FirewallRules: []string{},
		DeletionRules: []string{},
	}

	// Get all contexts
	allowedDomains := NewAllowedDomains(m.cluster.Spec.AdditionalDomains)
	contexts, err := m.listContexts(allowedDomains)
	if err != nil {
		return status, errors.Wrap(err, "failed to list contexts")
	}

	// Get the list of namespaces and check if the namespace exists
	// If the namespace does not exist, create it
	namespaces := &corev1.NamespaceList{}
	err = m.client.List(m.context, namespaces)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list namespaces")
	}

	namespaceCleanup := make(map[string]bool)
	// Create a namespace for each context
	for _, ctx := range contexts {

		config, err := ClientConfig(ctx.name, m.cluster.Spec.RemapToIp, m.getOptions())
		if err != nil {
			m.log.Error(err, "failed to get client config", "context", ctx.name)
			continue
		}

		var details *api.Config
		{
			switch config.provider {
			case ProviderKindAWS:
				details, err = aws.KubeConfig(ctx.cluster, config.Config)
			case ProviderKindClientCert:
				details, err = clientcert.KubeConfig(ctx.name, config.Config)
			default:
				m.log.Info("provider not supported", "provider", config.provider)
				continue
			}

			if err != nil {
				m.log.Error(err, "failed to get kubeconfig", "context", ctx.name)
				continue
			}
		}

		re := regexp.MustCompile("[^a-zA-Z0-9]+")
		var name = re.ReplaceAllString(ctx.name, "-")

		var namespaceName string = name
		namespacePrefix := m.cluster.Spec.NamespacePrefix
		if namespacePrefix != "" {
			namespacePrefix = strings.TrimSuffix(namespacePrefix, "-")
			namespaceName = namespacePrefix + "-" + name
		}

		// TODO: Future improvement: Check if the namespace needs deleting
		for _, ns := range namespaces.Items {
			if ns.Name == namespaceName {
				namespaceCleanup[ns.Name] = false
			}
		}

		err = m.createNamespaceForCluster(namespaceName, *namespaces)
		if err != nil {
			m.log.Error(err, "failed to create namespace", "namespace", ctx.name)
			continue
		}

		err = m.createSecretForCluster(namespaceName, name, details)
		if err != nil {
			m.log.Error(err, "failed to create secret", "namespace", namespaceName, "secret", ctx.name)
			continue
		}

		status.ClusterStatus[ctx.name] = kccnv1alpha1.ClusterStatusEntry{
			Ready:          m.clusterAvailable(namespaceName, ctx.name+"-kubeconfig"),
			Endpoint:       details.Clusters[ctx.name].Server,
			KubeConfig:     ctx.name + "-kubeconfig",
			LastUpdateTime: metav1.Now(),
		}

		// Only add rules if the original IP is different from the remapped IP
		if addr := net.ParseIP(config.originalIp); addr != nil && config.originalIp != config.remappedIp {
			if !status.ClusterStatus[ctx.name].Ready {
				rule := m.makeFirewallRule(config.originalIp, config.remappedIp, config.port)
				status.FirewallRules = append(status.FirewallRules, rule)
			}

			rule := m.makeDeleteFirewallRule(config.originalIp, config.remappedIp, config.port)
			status.DeletionRules = append(status.DeletionRules, rule)
		}
	}
	return status, nil
}

func (m *Manager) createNamespaceForCluster(clusterName string, namespaces corev1.NamespaceList) error {
	var exists bool
	for _, ns := range namespaces.Items {
		if ns.Name == clusterName {
			exists = true
			break
		}
	}

	if exists {
		return nil
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
	}

	err := m.client.Create(m.context, ns)
	if err != nil {
		return errors.Wrap(err, "failed to create namespace")
	}

	return nil
}

func (m *Manager) createSecretForCluster(namespace, clusterName string, details *api.Config) error {
	// Get the list of secrets and check if the secret exists
	// If the secret does not exist, create it
	secrets := &corev1.SecretList{}
	err := m.client.List(m.context, secrets)
	if err != nil {
		return errors.Wrap(err, "failed to list secrets")
	}

	secretName := clusterName + "-kubeconfig"

	var exists bool
	for _, secret := range secrets.Items {
		if secret.Name == secretName {
			exists = true
			break
		}
	}

	if !exists {
		_ = api.MinifyConfig(details)
		content, err := clientcmd.Write(*details)
		if err != nil {
			return errors.Wrap(err, "failed to write kubeconfig")
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"value": content,
			},
		}
		err = m.client.Create(m.context, secret)
		if err != nil {
			return errors.Wrap(err, "failed to create secret")
		}
	}

	return nil
}

func (m *Manager) listContexts(allowedDomains AllowedDomains) (contexts ContextList, err error) {
	options := m.getOptions()
	config, err := options.configAccess.GetStartingConfig()
	if err != nil {
		err = errors.Wrap(err, "cannot get starting config")
		return
	}

	for name, ctxConfig := range config.Contexts {
		cluster := config.Clusters[ctxConfig.Cluster]
		if !allowedDomains.Has(cluster.Server) {
			continue
		}

		ctx := Context{
			name:    name,
			user:    ctxConfig.AuthInfo,
			cluster: ctxConfig.Cluster,
		}
		contexts = append(contexts, ctx)
	}

	return
}

func (m *Manager) getOptions() GetContextsOptions {
	pathOptions := clientcmd.NewDefaultPathOptions()
	pathOptions.GlobalFile = m.cluster.Spec.KubeConfigPath
	pathOptions.EnvVar = ""

	return GetContextsOptions{configAccess: pathOptions}
}

func (m *Manager) clusterAvailable(namespace, secretName string) bool {
	var (
		err    error
		config *rest.Config
	)

	// Get secret
	secret := &corev1.Secret{}
	if err = m.client.Get(m.context, client.ObjectKey{Namespace: namespace, Name: secretName}, secret); err != nil {
		m.log.Error(err, "failed to get secret")
		return false
	}

	// Build config
	kubeconfig := secret.Data["value"]
	config, err = clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		m.log.Error(err, "failed to build config from kubeconfig", "kubeconfig", string(kubeconfig))
		return false
	}

	// Test connection
	client, err := client.New(config, client.Options{})
	if err != nil {
		m.log.Error(err, "failed to create clientset")
		return false
	}

	err = client.List(m.context, &corev1.NamespaceList{})
	if err != nil {
		m.log.Error(err, "failed to get server version")
		return false
	}

	return true
}
