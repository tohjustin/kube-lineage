# kubectl-lineage

[![build](https://github.com/tohjustin/kubectl-lineage/actions/workflows/build.yaml/badge.svg)](https://github.com/tohjustin/kubectl-lineage/actions/workflows/build.yaml)
[![release](https://aegisbadges.appspot.com/static?subject=release&status=v0.1.0&color=318FE0)](https://github.com/tohjustin/kubectl-lineage/releases)
[![kubernetes compatibility](https://aegisbadges.appspot.com/static?subject=k8s%20compatibility&status=v1.19%2B&color=318FE0)](https://endoflife.date/kubernetes)
[![license](https://aegisbadges.appspot.com/static?subject=license&status=Apache-2.0&color=318FE0)](./LICENSE.md)

A kubectl plugin to display all dependents of a Kubernetes object.

```shell
$ kubectl lineage node k3d-dev-server-0
NAMESPACE         NAME                           READY   STATUS         AGE
                  Node/k3d-dev-server-0          True    KubeletReady   30m
                  ├── CSINode/k3d-dev-server-0   -                      30m
kube-node-lease   └── Lease/k3d-dev-server-0     -                      30m

$ kubectl lineage svc/traefik
NAME                                                  READY   STATUS    AGE
Service/traefik                                       -                 30m
├── DaemonSet/svclb-traefik                           1/1               30m
│   ├── ControllerRevision/svclb-traefik-694565b64f   -                 30m
│   └── Pod/svclb-traefik-rrpdf                       2/2     Running   30m
└── EndpointSlice/traefik-klkwg                       -                 30m

$ kubectl lineage daemonset.apps svclb-traefik --show-group
NAME                                                   READY   STATUS    AGE
DaemonSet.apps/svclb-traefik                           1/1               30m
├── ControllerRevision.apps/svclb-traefik-694565b64f   -                 30m
└── Pod/svclb-traefik-rrpdf                            2/2     Running   30m
```

List of supported relationships used for discovering dependent objects:

- Kubernetes
  - [Controller References](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/controller-ref.md) & [Owner References](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/)
  - [Event References](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/event-v1/)
  - [Pod References](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/)
- Helm (Coming Soon)

## Installation

### Install from Source

```shell
git clone git@github.com:tohjustin/kubectl-lineage.git
make install

kubectl-lineage --version
```

## Prior Art

kubectl-lineage has been inspired by the following projects:

- [ahmetb/kubectl-tree](https://github.com/ahmetb/kubectl-tree)
- [nimakaviani/knative-inspect](https://github.com/nimakaviani/knative-inspect/)
- [steveteuber/kubectl-graph](https://github.com/steveteuber/kubectl-graph)
