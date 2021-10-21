# kube-lineage

[![build](https://github.com/tohjustin/kube-lineage/actions/workflows/build.yaml/badge.svg)](https://github.com/tohjustin/kube-lineage/actions/workflows/build.yaml)
[![release](https://aegisbadges.appspot.com/static?subject=release&status=v0.3.0&color=318FE0)](https://github.com/tohjustin/kube-lineage/releases)
[![kubernetes compatibility](https://aegisbadges.appspot.com/static?subject=k8s%20compatibility&status=v1.19%2B&color=318FE0)](https://endoflife.date/kubernetes)
[![helm compatibility](https://aegisbadges.appspot.com/static?subject=helm%20compatibility&status=v3&color=318FE0)](https://endoflife.date/kubernetes)
[![license](https://aegisbadges.appspot.com/static?subject=license&status=Apache-2.0&color=318FE0)](./LICENSE.md)

A CLI tool to display all dependents of an object in a Kubernetes cluster.

```shell
$ kube-lineage deploy/coredns
NAME                                           READY   STATUS    AGE
Deployment/coredns                             1/1               30m
└── ReplicaSet/coredns-5d69dc75db              1/1               30m
    └── Pod/coredns-5d69dc75db-26wjw           1/1     Running   30m
        └── Service/kube-dns                   -                 30m
            └── EndpointSlice/kube-dns-pxh5w   -                 30m

$ kube-lineage clusterrole/system:metrics-server --show-group -o wide
NAMESPACE     NAME                                                                           READY   STATUS    AGE    RELATIONSHIPS
              ClusterRole.rbac.authorization.k8s.io/system:metrics-server                    -                 30m   []
              └── ClusterRoleBinding.rbac.authorization.k8s.io/system:metrics-server         -                 30m   [ClusterRoleBindingRole]
kube-system       └── ServiceAccount/metrics-server                                          -                 30m   [ClusterRoleBindingSubject]
kube-system           ├── Pod/metrics-server-7b4f8b595-8m7rz                                 1/1     Running   30m   [PodServiceAccount]
kube-system           │   └── Service/metrics-server                                         -                 30m   [Service]
                      │       ├── APIService.apiregistration.k8s.io/v1beta1.metrics.k8s.io   True              30m   [APIService]
kube-system           │       └── EndpointSlice.discovery.k8s.io/metrics-server-wb2cm        -                 30m   [ControllerReference OwnerReference]
kube-system           └── Secret/metrics-server-token-nqw85                                  -                 30m   [ServiceAccountSecret]
kube-system               └── Pod/metrics-server-7b4f8b595-8m7rz                             1/1     Running   30m   [PodVolume]
```

Use either the `split` or `split-wide` output format to display dependents grouped by their type.

```shell
$ kube-lineage node k3d-dev-server-1 -o split
NAME                       DRIVERS   AGE
csinode/k3d-dev-server-1   0         25m

NAMESPACE           NAME                                         ADDRESSTYPE   PORTS   ENDPOINTS    AGE
monitoring-system   endpointslice/ksm-kube-state-metrics-r4dsh   IPv4          8080    10.42.0.10   25m

NAMESPACE           LAST SEEN   TYPE     REASON      OBJECT                                       MESSAGE
monitoring-system   4m40s       Normal   Scheduled   pod/ksm-kube-state-metrics-d785759dd-75gnc   Successfully assigned monitoring-system/ksm-kube-state-metrics-d785759dd-75gnc to k3d-dev-server-1
monitoring-system   4m40s       Normal   Pulling     pod/ksm-kube-state-metrics-d785759dd-75gnc   Pulling image "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.2.0"
monitoring-system   4m36s       Normal   Pulled      pod/ksm-kube-state-metrics-d785759dd-75gnc   Successfully pulled image "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.2.0" in 4.110982123s
monitoring-system   4m36s       Normal   Created     pod/ksm-kube-state-metrics-d785759dd-75gnc   Created container kube-state-metrics
monitoring-system   4m36s       Normal   Started     pod/ksm-kube-state-metrics-d785759dd-75gnc   Started container kube-state-metrics

NAME                    STATUS   ROLES    AGE    VERSION
node/k3d-dev-server-1   Ready    master   25m    v1.19.15+k3s2

NAMESPACE           NAME                                         READY   STATUS    RESTARTS   AGE
monitoring-system   pod/ksm-kube-state-metrics-d785759dd-75gnc   1/1     Running   0          25m

NAMESPACE           NAME                             TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
monitoring-system   service/ksm-kube-state-metrics   ClusterIP   10.43.153.203   <none>        8080/TCP   25m
```

Use the `helm` subcommand to display Helm release resources & their respective dependents in a Kubernetes cluster.

```shell
$ kube-lineage helm traefik -o wide
NAMESPACE     NAME                                                                      READY   STATUS     AGE    RELATIONSHIPS
kube-system   traefik                                                                   True    Deployed   30m    []
              ├── ClusterRole/traefik                                                   -                  30m    [HelmRelease]
              │   └── ClusterRoleBinding/traefik                                        -                  30m    [ClusterRoleBindingRole]
kube-system   │       └── ServiceAccount/traefik                                        -                  30m    [ClusterRoleBindingSubject]
kube-system   │           ├── Pod/traefik-5dd496474-cr6d8                               1/1     Running    30m    [PodServiceAccount]
kube-system   │           │   ├── Service/traefik                                       -                  30m    [Service]
kube-system   │           │   │   ├── DaemonSet/svclb-traefik                           1/1                30m    [ControllerReference OwnerReference]
kube-system   │           │   │   │   ├── ControllerRevision/svclb-traefik-5b8d578897   -                  30m    [ControllerReference OwnerReference]
kube-system   │           │   │   │   └── Pod/svclb-traefik-8vfqq                       2/2     Running    30m    [ControllerReference OwnerReference]
kube-system   │           │   │   └── EndpointSlice/traefik-lns42                       -                  30m    [ControllerReference OwnerReference]
kube-system   │           │   └── Service/traefik-prometheus                            -                  30m    [Service]
kube-system   │           │       └── EndpointSlice/traefik-prometheus-c72cc            -                  30m    [ControllerReference OwnerReference]
kube-system   │           └── Secret/traefik-token-qgfn9                                -                  30m    [ServiceAccountSecret]
kube-system   │               └── Pod/traefik-5dd496474-cr6d8                           1/1     Running    30m    [PodVolume]
              ├── ClusterRoleBinding/traefik                                            -                  30m    [HelmRelease]
kube-system   ├── ConfigMap/traefik                                                     -                  30m    [HelmRelease]
kube-system   │   └── Pod/traefik-5dd496474-cr6d8                                       1/1     Running    30m    [PodVolume]
kube-system   ├── ConfigMap/traefik-test                                                -                  30m    [HelmRelease]
kube-system   ├── Deployment/traefik                                                    1/1                30m    [HelmRelease]
kube-system   │   └── ReplicaSet/traefik-5dd496474                                      1/1                30m    [ControllerReference OwnerReference]
kube-system   │       └── Pod/traefik-5dd496474-cr6d8                                   1/1     Running    30m    [ControllerReference OwnerReference]
kube-system   ├── Secret/sh.helm.release.v1.traefik.v1                                  -                  30m    [HelmStorage]
kube-system   ├── Secret/traefik-default-cert                                           -                  30m    [HelmRelease]
kube-system   │   └── Pod/traefik-5dd496474-cr6d8                                       1/1     Running    30m    [PodVolume]
kube-system   ├── Service/traefik                                                       -                  30m    [HelmRelease]
kube-system   ├── Service/traefik-prometheus                                            -                  30m    [HelmRelease]
kube-system   └── ServiceAccount/traefik                                                -                  30m    [HelmRelease]

$ kube-lineage helm kube-state-metrics --depth 1 -n monitoring-system -L app.kubernetes.io/managed-by -L owner
NAMESPACE           NAME                                                  READY   STATUS     AGE   MANAGED-BY   OWNER
monitoring-system   kube-state-metrics                                    True    Deployed   25m
                    ├── ClusterRole/kube-state-metrics                    -                  25m   Helm
                    ├── ClusterRoleBinding/kube-state-metrics             -                  25m   Helm
monitoring-system   ├── Deployment/kube-state-metrics                     1/1                25m   Helm
monitoring-system   ├── Secret/sh.helm.release.v1.kube-state-metrics.v1   -                  25m                helm
monitoring-system   ├── Service/kube-state-metrics                        -                  25m   Helm
monitoring-system   └── ServiceAccount/kube-state-metrics                 -                  25m   Helm
```

List of supported relationships used for discovering dependent objects:

- Kubernetes
  - [APIService References](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/api-service-v1/)
  - [ClusterRole References](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-v1/), [ClusterRoleBinding References](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/), [Role References](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-v1/) & [RoleBinding References](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/)
  - [Controller References](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/controller-ref.md) & [Owner References](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/)
  - [CSINode References](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/csi-node-v1/)
  - [CSIStorageCapacity References](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/csi-storage-capacity-v1beta1/)
  - [Event References](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/event-v1/)
  - [Ingress References](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/ingress-v1/) & [IngressClass References](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/ingress-class-v1/)
  - [MutatingWebhookConfiguration References](https://kubernetes.io/docs/reference/kubernetes-api/extend-resources/mutating-webhook-configuration-v1/) & [ValidatingWebhookConfiguration References](https://kubernetes.io/docs/reference/kubernetes-api/extend-resources/validating-webhook-configuration-v1/)
  - [NetworkPolicy References](https://kubernetes.io/docs/reference/kubernetes-api/policy-resources/network-policy-v1/)
  - [PersistentVolume References](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/persistent-volume-v1/) & [PersistentVolumeClaim References](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/persistent-volume-claim-v1/)
  - [Pod References](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/)
  - [PodDisruptionBudget References](https://kubernetes.io/docs/reference/kubernetes-api/policy-resources/pod-disruption-budget-v1/)
  - [PodSecurityPolicy References](https://kubernetes.io/docs/reference/kubernetes-api/policy-resources/pod-disruption-budget-v1/)
  - [RuntimeClass References](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/runtime-class-v1/)
  - [Service References](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/service-v1/)
  - [ServiceAccount References](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/service-account-v1/)
  - [StorageClass References](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/storage-class-v1/)
  - [VolumeAttachment References](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/volume-attachment-v1/)
- Helm
  - [Release References](https://helm.sh/docs/intro/using_helm/#three-big-concepts) & [Storage References](https://helm.sh/docs/topics/advanced/#storage-backends)

## Installation

### Install via [krew](https://krew.sigs.k8s.io/)

```shell
kubectl krew install lineage
```

### Install from Source

```shell
git clone git@github.com:tohjustin/kube-lineage.git
make install

kube-lineage --version
```

## Prior Art

kube-lineage has been inspired by the following projects:

- [ahmetb/kubectl-tree](https://github.com/ahmetb/kubectl-tree)
- [feloy/kubectl-service-tree](https://github.com/feloy/kubectl-service-tree)
- [nimakaviani/knative-inspect](https://github.com/nimakaviani/knative-inspect/)
- [steveteuber/kubectl-graph](https://github.com/steveteuber/kubectl-graph)
