# Kube-Apiserver Backed Apiserver

**Project status: proof of concept**

Setting up an [Aggregation Apiserver](https://kubernetes.io/docs/tasks/access-kubernetes-api/configure-aggregation-layer/) for managed kubernetes service can be tedious. [apiserver-builder](https://github.com/kubernetes-sigs/apiserver-builder-alpha) greatly reduce the work of building an apiserver, but spinning up a storage backend for this apiserver can be a more complicated problem.

This project plumbs aggregated apiserver over the kube-apiserver, namely, using custom resource as the storage backend.

Supported storage interfaces:

- [x] Create
- [x] Get
- [x] GetToList
- [x] Delete
- [x] List
- [ ] GuaranteedUpdate
- [ ] Watch
- [ ] WatchList

## Test locally

Install [apiserver-builder](https://github.com/kubernetes-sigs/apiserver-builder-alpha) first.

```shell
$ apiserver-boot init dep
$ apiserver-boot build depdencies
$ go build -o bin/apiserver cmd/apiserver/main.go

$ export KUBECONFIG=~/.kube/config
$ ./bin/apiserver --secure-port=9443 --delegated-auth=false

$ kubectl --kubeconfig kubeconfig api-resources
$ kubectl --kubeconfig kubeconfig create tidbconfigs sample/tidbconfig.yaml
$ kubectl --kubeconfig kubeconfig get tidbconfigs tidbconfig-example
```

