# servicebinding-runtime

## Description
Experimental implementation of the [ServiceBinding.io](https://servicebinding.io) [1.0 spec](https://servicebinding.io/spec/core/1.0.0/) using [reconciler-runtime](https://github.com/vmware-labs/reconciler-runtime/). The full specification is implemented, please open an issue for any discrepancies.

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Define where to publish images:

```sh
export KO_DOCKER_REPO=<a-repository-you-can-write-to>
```

For kind, a registry is not required:

```sh
export KO_DOCKER_REPO=kind.local/servicebinding
```
	
1. Build and Deploy the controller to the cluster:

```sh
make deploy
```

### Undeploy controller
Undeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing

### Test It Out

1. Run the unit tests:

```sh
make test
```

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2022 Scott Andrews.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

