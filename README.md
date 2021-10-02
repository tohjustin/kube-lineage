# kubectl-lineage

[![build](https://github.com/tohjustin/kubectl-lineage/actions/workflows/build.yaml/badge.svg)](https://github.com/tohjustin/kubectl-lineage/actions/workflows/build.yaml)
[![release](https://aegisbadges.appspot.com/static?subject=release&status=v0.1.0&color=318FE0)](https://github.com/tohjustin/kubectl-lineage/releases)
[![kubernetes compatibility](https://aegisbadges.appspot.com/static?subject=k8s%20compatibility&status=v1.19%2B&color=318FE0)](https://endoflife.date/kubernetes)
[![license](https://aegisbadges.appspot.com/static?subject=license&status=Apache-2.0&color=318FE0)](./LICENSE.md)

A kubectl plugin to display all dependents of a Kubernetes object.

```shell
$ kubectl lineage deploy/coredns
NAME                                           READY   STATUS    AGE
Deployment/coredns                             1/1               30m
└── ReplicaSet/coredns-5d69dc75db              1/1               30m
    └── Pod/coredns-5d69dc75db-26wjw           1/1     Running   30m
        └── Service/kube-dns                   -                 30m
            └── EndpointSlice/kube-dns-pxh5w   -                 30m

$ kubectl lineage node k3d-dev-server-1 -o wide
NAMESPACE           NAME                                                 READY   STATUS         AGE   RELATIONSHIPS
                    Node/k3d-dev-server-1                                True    KubeletReady   30m   -
                    ├── CSINode/k3d-dev-server-1                         -                      30m   [OwnerReference]
kube-node-lease     ├── Lease/k3d-dev-server-1                           -                      30m   [OwnerReference]
kube-system         ├── Pod/metrics-server-7b4f8b595-mxtfp               1/1     Running        30m   [PodNode]
kube-system         │   └── Service/metrics-server                       -                      30m   [Service]
kube-system         │       └── EndpointSlice/metrics-server-lbhb9       -                      30m   [ControllerReference OwnerReference]
monitoring-system   └── Pod/kube-state-metrics-6cb9b94fdf-bkz22          1/1     Running        25m   [PodNode]
monitoring-system       └── Service/kube-state-metrics                   -                      25m   [Service]
monitoring-system           └── EndpointSlice/kube-state-metrics-zkggx   -                      25m   [ControllerReference OwnerReference]

$ kubectl clusterrole/system:metrics-server -o wide
NAMESPACE     NAME                                                         READY   STATUS    AGE   RELATIONSHIPS
              ClusterRole/system:metrics-server                            -                 30m   -
              └── ClusterRoleBinding/system:metrics-server                 -                 30m   [ClusterRoleBindingRole]
kube-system       └── ServiceAccount/metrics-server                        -                 30m   [ClusterRoleBindingSubject]
kube-system           └── Secret/metrics-server-token-sz96w                -                 30m   [ServiceAccountSecret]
kube-system               └── Pod/metrics-server-7b4f8b595-mxtfp           1/1     Running   30m   [PodVolume]
kube-system                   └── Service/metrics-server                   -                 30m   [Service]
kube-system                       └── EndpointSlice/metrics-server-lbhb9   -                 30m   [ControllerReference OwnerReference]
```

List of supported relationships used for discovering dependent objects:

- Kubernetes
  - [ClusterRole References](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-v1/), [ClusterRoleBinding References](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) & [RoleBinding References](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/)
  - [Controller References](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/controller-ref.md) & [Owner References](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/)
  - [Event References](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/event-v1/)
  - [Ingress References](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/ingress-v1/) & [IngressClass Reference](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/ingress-class-v1/)
  - [MutatingWebhookConfiguration References](https://kubernetes.io/docs/reference/kubernetes-api/extend-resources/mutating-webhook-configuration-v1/) & [ValidatingWebhookConfiguration References](https://kubernetes.io/docs/reference/kubernetes-api/extend-resources/validating-webhook-configuration-v1/)
  - [PersistentVolume References](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/persistent-volume-v1/) & [PersistentVolumeClaim References](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/persistent-volume-claim-v1/)
  - [Pod References](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/)
  - [Service References](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/service-v1/)
  - [ServiceAccount References](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/service-account-v1/)
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
