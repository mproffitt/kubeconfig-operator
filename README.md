# EXPERIMENTAL: kubeconfig-operator

Turn a kind cluster into a management cluster.

`kubeconfig-operator` is a simple operator/controller for mounting kubeconfigs
into a local development cluster.

## Description

Have you ever tried developing the hub-spoke model locally and got fed up with
the tedious setup of extracting contexts from your kubeconfig to mount them
as secrets into the cluster you're using as a management cluster?

You may well have never tried because it just feels like a lot of effort.

This is where `kubeconfig-operator` comes in. I wrote this operator because I
work with multi-cluster setups every day and trying to set all that up locally
is painful.

> [!Caution]
> This is a development only tool. It's sole purpose is to work with your
> kubeconfig and mount each local context as a secret inside the cluster where
> this operator is running.
>
> It is not designed for production use and using it in such an environment
> is highly discouraged and likely won't work for you.

The operator works by mounting the `kubeconfig` path you specify, then scanning
each context. If the context server path URI in the list of allowed paths, which
by default are `localhost`, `localhost.localdomain`, `127.0.0.1` and
`localhost.localstack.cloud` then that context is considered a candidate for
mounting.

The operator will then create a namespace for that context and create a secret
containing the kubeconfig entry. By default, namespaces are prefixed with
`cluster-` to identify them. Secrets will always have the suffix `-kubeconfig`.

> [!Tip]
> When working with `localstack` EKS clusters, by default the `aws` CLI
> creates kubeconfig entries that are referenced by ARN. It is recommended
> that you set an alias for these instead to avoid long namespace and secret
> names.
>
> If you do not set an alias, the arn will be sanitized to become hyphen (`-`)
> separated strings.
>
> For example `arn:aws:eks:us:east:1:000000000000:cluster/eks1` will become
> `arn-aws-eks-us-east-1-000000000000-cluster-eks1`

## Getting Started

To use the operator, we first need a cluster to use as a management (hub)
cluster and a couple of additional clusters to use as workload clusters.

In this example we will use a 3 node kind cluster with a node selector label
to target the node we want the operator to run on. In to each cluster we are
spinning up, we need to alter the default `kubeadmr` certificates and add the IP
address assigned to our external interface.

To make this easier, the following may also be found in
[`kindconfig/management.yaml`](./kindconfig/management.yaml) and
[`kindcluster/tenant.yaml`](./kindconfig/tenant.yaml) respectively.

The management cluster has a label selector for the operator, and a hostpath
mount to enable passing through the kubeconfig. These are attached to the
control-plane only. If you add additional control-planes, remember to copy the
extra config to each one.

Modify the [kindconfig/management.yaml](./kindconfig/management.yaml) file and
change the `certSANS` list to match your envinronment.

> [!Warning]
> **DO NOT** remove the entry for `127.0.0.1`. This is required to access the
> cluster from your local machine.

The management-cluster config file has a name attached into it. It is recommended
that you leave this as is for the moment. Once you're familiar with how the
operator works then feel free to change it however you require.

Create the cluster with:

```sh
kind create cluster --config kindconfig/management.yaml`
```

### Creating tenant clusters

Similar to the management cluster, the workload or tenant clusters must also
have their `certSANS` patched with the external IP.

The tenant clusters name will be specified on the command line as each one will
be different.

We also need a few tenant clusters (spokes) to test with. Again using kind, lets
create 2 additional clusters.

```sh
for i in $(seq 1 2); do
    kind create cluster --name "tenant$i" --config kindconfig/tenant.yaml
done
```

> [!Note]
> Remember to switch back to the management cluster context before continuing.
>
> ```sh
> kubectl config use-context kind-management-cluster
> ```

## Quickstart

To get up and running with the operator quickly, first deploy it to the cluster
with:

```sh
make install
make deploy IMG=docker.io/choclab/kubeconfig-operator:latest
```

Next, edit [`config/samples/v1alpha1_cluter.yaml`](config/samples/v1alpha1_cluter.yaml)
and configure it as follows:

- `additionalDomains` A list of additional domains or IPs that you use for
  local clusters. Normally you can leave this empty. This list is merged with
  the default set of `localhost`, `localhost.localdomain`, `127.0.0.1` and
  `localhost.localstack.cloud`.
- `firewallFormat` This is the format to print firwall rules for. Current accepted
  values are:

  - `iptables` This is the default format
  - `nftables`
  - `ufw`
  - `firewalld`
  - `ipfw`
  - `pf`

  Please see the note below in relation to the firewall rules.
- `kubeConfigPath` This is the path on the pod to load the kubeconfig from.
  Leave this as `/tmp/kubeconfig` unless you're extending the deployment to
  accept multiple kubeconfigs. In which case, create one cluster object per
  kubeconfig path
- `namespacePrefix` When namespaces are created, they will be prefixed with this
  string. By default this is set to `cluster`
- `remapToIp` This should be the address of your external ethernet device and will
  normally be a `192.168.0.0/16` address.
- `reconcileInterval` The interval at which clusters in this kubeconfig will be
  reconciled. Default for reconciliation is 30s to be responsive to new clusters
  being added to the config
- `suspend` If true, clusters from this kubeconfig will not be reconciled

> [!Note]
> If a cluster is unreachable from the management cluster, a set of firewall
> rules will be printed to the status of the CR.
>
> With the exception of `iptables` rules, these are auto-generated and may not
> have been tested.
>
> If you find an error in any of the rules generated, please raise an issue
> or PR with the correction.

Once you have edited the CR, this can be applied with

```bash
kubectl apply -k config/samples/
```

> [!Important]
> Before moving on to the next section, make sure you have read and understood
> the security implications in using `net.ipv4.conf.all.route_localnet=1`
>
> This section relies upon this behaviour being enabled in `sysctl` on Linux
> but raises security concerns in that it may cause local services to be exposed
> on the public interface.
>
> It is **not** recommended to enable this by default, but only enable it when
> you need to work in a multicluster setup, and disable it afterwards.

### Apply firewall rules

> [!Caution]
> Local clusters such as those generated by `kind`, `minikube`, `localstack`
> or others are **not** meant to be exposed publically.
>
> The rules generated are specifically designed to bind between your external
> interface and `localhost` so that the clusters can be reachable from inside
> the management cluster.
>
> It is **not recommended** that you expose these publically but **is recommended**
> that you use your routers firewall to block traffic.
>
> As an alternative, you may want to consider adding a virtual interface and
> using the address for that as the `RemapToIp` address.
>
> This may be especially useful when connecting on wireless networks in a public
> environment.
>
> You can find instructions on adding a virtual network device on Linux at
> <https://linuxconfig.org/configuring-virtual-network-interfaces-in-linux>

Once the CR is created and starts reconciling, you may find that clusters are
unreachable.

Unreachable clusters show as `ready: false` in the status of the CR. You can
check this with

```bash
kubectl get clusters.kubeconfig.choclab.net cluster-sample -o yaml \
  | yq ' select(.status.clusters[].ready == false)'
```

Unready clusters will also have firewall rules in the `.status.firewallRules`
array which can be applied as follows (Linux)

```bash
readarray rules < <(kubectl get cluster cluster-sample -o yaml \
  | yq -r -o=j -I=0 .status.firewallRules[])
for rule in "${rules[@]}"; do sudo $rule; done
```

On linux, you will also need to run the following command to enable local routing
to be exposed:

```bash
sudo sysctl -w net.ipv4.conf.all.route_localnet=1
```

A set of deletion rules are also provided in `status.deletionRules` and may be
equivelantly applied with:

```bash
readarray rules < <(kubectl get cluster cluster-sample -o yaml \
  | yq -r -o=j -I=0 .status.deletionRules[])
for rule in "${rules[@]}"; do sudo $rule; done
```

> [!Tip]
> Deletion rules are always present in the status of the CR whilst firewall
> rules are only present if the cluster is unreachable.

#### Working with Localstack

If working with `localstack` EKS instances, firewall rules are **not** generated
for them.

Localstack uses a fixed set of ports for all of its services and therefore these
rules may need to be applied seperately.

First, make sure you have a hostfile entry for `localhost.localstack.cloud` which
points at the IP of the interface you wish to use. You can then apply the
following loop to create the rules for you.

```sh
EXTERN_ADDR="192.168.1.2"
for i in $(seq 4510 4560) 4566; do
  sudo iptables -t nat -A PREROUTING -p tcp -d $EXTERN_ADDR/32 --dport $i -j DNAT --to-destination 127.0.0.1:$i;
done
```

Set `EXTERN_ADDR` to your external interface address.

Equivelantly deletion can be done with

```sh
EXTERN_ADDR="192.168.1.2"
for i in $(seq 4510 4560) 4566; do
  sudo iptables -t nat -D PREROUTING -p tcp -d $EXTERN_ADDR/32 --dport $i -j DNAT --to-destination 127.0.0.1:$i;
done
```

### Sample Workloads

Now we can create a sample workload that targets one of the clusters to verify
everything is working as we expect.

First, lets install `fluxcd` to the management cluster.

> [!Note]
> If you don't already have the Flux CLI command, you can install it by following
> the instructions here: <https://fluxcd.io/flux/installation/#install-the-flux-cli>

Next, install the controllers to the management cluster with:

```sh
kubectl config use-context kind-management-cluster
flux install
```

If you want the additional image controllers they can be installed by using

```sh
flux install --components-extra="image-reflector-controller,image-automation-controller"
```

From the config/samples directory, install the helmrelease for podinfo. By
default this will target the `tenant1` cluster. If you change this, don't forget
to change the namespace as well.

```sh
k apply -f config/samples/podinfo.yaml
```

If everything has worked correctly, you can check the status of the HelmRelease
and should now see it deployed to the `tenant1` cluster.

```sh
➜  kubeconfig-operator git:(main) ✗ k get hr -n cluster-kind-tenant1
NAME      AGE   READY   STATUS
podinfo   1m   True    Helm install succeeded for release default/default-podinfo.v1 with chart podinfo@6.5.4
```

We can confirm this on the tenant cluster:

```sh
➜  kubeconfig-operator git:(main) ✗ k --context kind-tenant1 get po -n default
NAME                               READY   STATUS    RESTARTS   AGE
default-podinfo-655565bcfb-djbdp   1/1     Running   0          1m
default-podinfo-655565bcfb-zn6hq   1/1     Running   0          1m
```

And with that, we now have a multi-cluster operator working.

If you add a new workload/tenent cluster to your kubeconfig, this should be reflected back into the management cluster within a few seconds.

```sh
➜  kubeconfig-operator git:(main) ✗ kind create cluster --name example --config kindconfig/tenant.yaml; k get ns -w
Creating cluster "example" ...
 ✓ Ensuring node image (kindest/node:v1.29.2) 🖼
 ✓ Preparing nodes 📦
 ✓ Writing configuration 📜
 ✓ Starting control-plane 🕹️
 ✓ Installing CNI 🔌
 ✓ Installing StorageClass 💾
Set kubectl context to "kind-example"
You can now use your cluster with:

kubectl cluster-info --context kind-example

Have a question, bug, or feature request? Let us know! https://kind.sigs.k8s.io/#community 🙂

➜  kubeconfig-operator git:(main) ✗ k get ns --context kind-management-cluster | grep cluster
NAME                              STATUS   AGE
cluster-kind-example              Active   5s
cluster-kind-management-cluster   Active   10m
cluster-kind-tenant1              Active   10m
cluster-kind-tenant2              Active   10m
```

And of course we can do the same thing for `localstack` EKS clusters

```sh
➜  kubeconfig-operator git:(main) ✗ awslocal eks create-cluster \
  --name testeks \
  --role-arn "arn:aws:iam::000000000000:role/eks-role" \
  --resources-vpc-config "{}" --kubernetes-version 1.29
➜  kubeconfig-operator git:(main) ✗ awslocal eks update-kubeconfig --name testeks --alias testeks
➜  kubeconfig-operator git:(main) ✗ k get ns --context kind-management-cluster | grep cluster
NAME                              STATUS   AGE
cluster-kind-example              Active   15m
cluster-kind-management-cluster   Active   16m
cluster-kind-tenant1              Active   16m
cluster-kind-tenant2              Active   16m
cluster-testeks                   Active   1s
```

> [!Note]
> EKS clusters may or may not be reachable at the moment but this **may** be a
> product of my local environment.
>
> Sometimes, the proxy doesn't seem to start properly. I am working on
> understanding why this is in order to present a consistent solution.

## Building

### Prerequisites

- go version v1.23.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster

**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/kubeconfig-operator:tag
```

> [!Note] This image ought to be published in the personal registry you
> specified and it is required to have access to pull the image from the
> working environment.
>
> Make sure you have the proper permission to the registry if the above commands
> don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/kubeconfig-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall

**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/kubeconfig-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/kubeconfig-operator/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing

// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

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
