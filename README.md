# kube-lineage

[![build](https://github.com/tohjustin/kube-lineage/actions/workflows/build.yaml/badge.svg)](https://github.com/tohjustin/kube-lineage/actions/workflows/build.yaml)
[![release](https://aegisbadges.appspot.com/static?subject=release&status=v0.5.0&color=318FE0)](https://github.com/tohjustin/kube-lineage/releases)
[![kubernetes compatibility](https://aegisbadges.appspot.com/static?subject=k8s%20compatibility&status=v1.21%2B&color=318FE0)](https://endoflife.date/kubernetes)
[![helm compatibility](https://aegisbadges.appspot.com/static?subject=helm%20compatibility&status=v3&color=318FE0)](https://helm.sh/docs/topics/v2_v3_migration)
[![license](https://aegisbadges.appspot.com/static?subject=license&status=Apache-2.0&color=318FE0)](./LICENSE.md)

A CLI tool to display all dependencies or dependents of an object in a Kubernetes cluster.

## Usage

```shell
$ kube-lineage clusterrole system:metrics-server --output=wide
NAMESPACE     NAME                                                               READY   STATUS    AGE   RELATIONSHIPS
              ClusterRole/system:metrics-server                                  -                 30m   []
              └── ClusterRoleBinding/system:metrics-server                       -                 30m   [ClusterRoleBindingRole]
kube-system       └── ServiceAccount/metrics-server                              -                 30m   [ClusterRoleBindingSubject]
kube-system           ├── Pod/metrics-server-7b4f8b595-8m7rz                     1/1     Running   30m   [PodServiceAccount]
kube-system           │   └── Service/metrics-server                             -                 30m   [Service]
                      │       ├── APIService/v1beta1.metrics.k8s.io              True              30m   [APIService]
kube-system           │       └── EndpointSlice.discovery/metrics-server-wb2cm   -                 30m   [ControllerReference OwnerReference]
kube-system           └── Secret/metrics-server-token-nqw85                      -                 30m   [ServiceAccountSecret]
kube-system               └── Pod/metrics-server-7b4f8b595-8m7rz                 1/1     Running   30m   [PodVolume]
```

Use either the `--dependencies` or `-D` flag to show dependencies instead of dependents

```shell
$ kube-lineage pod coredns-5cc79d4bf5-xgvkc --dependencies
NAMESPACE     NAME                                                                   READY   STATUS         AGE
kube-system   Pod/coredns-5cc79d4bf5-xgvkc                                           1/1     Running        30m
              ├── Node/k3d-server                                                    True    KubeletReady   30m
              ├── PodSecurityPolicy/system-unrestricted-psp                          -                      30m
kube-system   ├── ConfigMap/coredns                                                  -                      30m
kube-system   ├── ReplicaSet/coredns-5cc79d4bf5                                      1/1                    30m
kube-system   │   └── Deployment/coredns                                             1/1                    30m
kube-system   ├── Secret/coredns-token-6vsx4                                         -                      30m
kube-system   │   └── ServiceAccount/coredns                                         -                      30m
              │       ├── ClusterRoleBinding/system:basic-user                       -                      30m
              │       │   └── ClusterRole/system:basic-user                          -                      30m
              │       ├── ClusterRoleBinding/system:coredns                          -                      30m
              │       │   └── ClusterRole/system:coredns                             -                      30m
              │       ├── ClusterRoleBinding/system:discovery                        -                      30m
              │       │   └── ClusterRole/system:discovery                           -                      30m
              │       ├── ClusterRoleBinding/system:public-info-viewer               -                      30m
              │       │   └── ClusterRole/system:public-info-viewer                  -                      30m
kube-system   │       └── RoleBinding/system-unrestricted-svc-acct-psp-rolebinding   -                      30m
              │           └── ClusterRole/system-unrestricted-psp-role               -                      30m
              │               └── PodSecurityPolicy/system-unrestricted-psp          -                      30m
kube-system   └── ServiceAccount/coredns                                             -                      30m
```

Use the `helm` subcommand to display Helm release resources & optionally their respective dependents in a Kubernetes cluster.

```shell
$ kube-lineage helm kube-state-metrics -n monitoring-system
helm kube-state-metrics -n monitoring-system
NAMESPACE           NAME                                                             READY   STATUS     AGE
monitoring-system   kube-state-metrics                                               True    Deployed   25m
                    ├── ClusterRole/kube-state-metrics                               -                  25m
                    │   └── ClusterRoleBinding/kube-state-metrics                    -                  25m
monitoring-system   │       └── ServiceAccount/kube-state-metrics                    -                  25m
monitoring-system   │           ├── Pod/kube-state-metrics-7dff544777-jb2q2          1/1     Running    25m
monitoring-system   │           │   └── Service/kube-state-metrics                   -                  25m
monitoring-system   │           │       └── EndpointSlice/kube-state-metrics-rq8wk   -                  25m
monitoring-system   │           └── Secret/kube-state-metrics-token-bsr4q            -                  25m
monitoring-system   │               └── Pod/kube-state-metrics-7dff544777-jb2q2      1/1     Running    25m
                    ├── ClusterRoleBinding/kube-state-metrics                        -                  25m
monitoring-system   ├── Deployment/kube-state-metrics                                1/1                25m
monitoring-system   │   └── ReplicaSet/kube-state-metrics-7dff544777                 1/1                25m
monitoring-system   │       └── Pod/kube-state-metrics-7dff544777-jb2q2              1/1     Running    25m
monitoring-system   ├── Secret/sh.helm.release.v1.kube-state-metrics.v1              -                  25m
monitoring-system   ├── Service/kube-state-metrics                                   -                  25m
monitoring-system   └── ServiceAccount/kube-state-metrics

$ kube-lineage helm traefik --depth 1 --label-columns app.kubernetes.io/managed-by --label-columns owner
NAMESPACE     NAME                                       READY   STATUS     AGE   MANAGED-BY   OWNER
kube-system   traefik                                    True    Deployed   30m
              ├── ClusterRole/traefik                    -                  30m   Helm
              ├── ClusterRoleBinding/traefik             -                  30m   Helm
kube-system   ├── ConfigMap/traefik                      -                  30m   Helm
kube-system   ├── ConfigMap/traefik-test                 -                  30m   Helm
kube-system   ├── Deployment/traefik                     1/1                30m   Helm
kube-system   ├── Secret/sh.helm.release.v1.traefik.v1   -                  30m                helm
kube-system   ├── Secret/traefik-default-cert            -                  30m   Helm
kube-system   ├── Service/traefik                        -                  30m   Helm
kube-system   ├── Service/traefik-prometheus             -                  30m   Helm
kube-system   └── ServiceAccount/traefik                 -                  30m   Helm
```

Use either the `split` or `split-wide` output format to display resources grouped by their type.

```shell
$ kube-lineage deploy/coredns --output=split --show-group
NAME                      READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/coredns   3/3     3            3           30m

NAME                                            ADDRESSTYPE   PORTS        ENDPOINTS                          AGE
endpointslice.discovery.k8s.io/kube-dns-mz9bw   IPv4          53,9153,53   10.42.0.24,10.42.0.26,10.42.0.27   30m

NAME                           READY   STATUS    RESTARTS   AGE
pod/coredns-5cc79d4bf5-xgvkc   1/1     Running   0          30m
pod/coredns-5cc79d4bf5-rjc7d   1/1     Running   0          30m
pod/coredns-5cc79d4bf5-tt2zl   1/1     Running   0          30m

NAME                                 DESIRED   CURRENT   READY   AGE
replicaset.apps/coredns-5cc79d4bf5   3         3         3       30m

NAME               TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)                  AGE
service/kube-dns   ClusterIP   10.43.0.10   <none>        53/UDP,53/TCP,9153/TCP   30m
```

### Flags

Flags for configuring relationship discovery parameters

| Flag | Description |
| ---- | ----------- |
| `--all-namespaces`, `-A` | If present, list object relationships across all namespaces |
| `--dependencies`, `-D`   | If present, list object dependencies instead of dependents. <br/> Not supported in `helm` subcommand |
| `--depth`, `-d`          | Maximum depth to find relationships |
| `--exclude-types`        | Accepts a comma separated list of resource types to exclude from relationship discovery. <br/> You can also use multiple flag options like --exclude-types type1 --exclude-types type2... |
| `--include-types`        | Accepts a comma separated list of resource types to only include in relationship discovery. <br/> You can also use multiple flag options like --include-types type1 --include-types type2... |
| `--scopes`, `-S`         | Accepts a comma separated list of additional namespaces to find relationships. <br/> You can also use multiple flag options like -S namespace1 -S namespace2... |

Flags for configuring output format

| Flag | Description |
| ---- | ----------- |
| `--output`, `-o`        | Output format. One of: wide \| split \| split-wide |
| `--label-columns`, `-L` | Accepts a comma separated list of labels that are going to be presented as columns. <br/> You can also use multiple flag options like -L label1 -L label2... |
| `--no-headers`          | When using the default output format, don't print headers |
| `--show-group`          | If present, include the resource group for the requested object(s) |
| `--show-label`          | When printing, show all labels as the last column |
| `--show-namespace`      | When printing, show namespace as the first column |

Use the following commands to view the full list of supported flags

```shell
$ kube-lineage --help
$ kube-lineage helm --help
```

## Supported Relationships

List of supported relationships used for discovering dependent objects:

- Kubernetes
  - [Controller](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/controller-ref.md) & [Owner](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/) References
  - Core APIs: [Event](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/event-v1/), [PersistentVolume](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/persistent-volume-v1/), [PersistentVolumeClaim](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/persistent-volume-claim-v1/), [Pod](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/), [Service](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/service-v1/), [ServiceAccount](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/service-account-v1/)
  - `policy` APIs: [PodDisruptionBudget](https://kubernetes.io/docs/reference/kubernetes-api/policy-resources/pod-disruption-budget-v1), [PodSecurityPolicy](https://kubernetes.io/docs/reference/kubernetes-api/policy-resources/pod-disruption-budget-v1/)
  - `admissionregistration.k8s.io` APIs: [MutatingWebhookConfiguration](https://kubernetes.io/docs/reference/kubernetes-api/extend-resources/mutating-webhook-configuration-v1/) & [ValidatingWebhookConfiguration](https://kubernetes.io/docs/reference/kubernetes-api/extend-resources/validating-webhook-configuration-v1/)
  - `apiregistration.k8s.io` APIs: [APIService](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/api-service-v1/)
  - `networking.k8s.io` APIs: [Ingress](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/ingress-v1/), [IngressClass](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/ingress-class-v1/), [NetworkPolicy](https://kubernetes.io/docs/reference/kubernetes-api/policy-resources/network-policy-v1/)
  - `node.k8s.io` APIs: [RuntimeClass](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/runtime-class-v1/)
  - `rbac.authorization.k8s.io` APIs: [ClusterRole](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-v1/), [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/), [Role](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-v1/), [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/)
  - `storage.k8s.io` APIs: [CSINode](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/csi-node-v1/), [CSIStorageCapacity](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/csi-storage-capacity-v1beta1/), [StorageClass](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/storage-class-v1/), [VolumeAttachment](https://kubernetes.io/docs/reference/kubernetes-api/config-and-storage-resources/volume-attachment-v1/)
- Helm
  - [Helm Release](https://helm.sh/docs/intro/using_helm/#three-big-concepts)
  - [Helm Storage](https://helm.sh/docs/topics/advanced/#storage-backends)

## Installation

### Install via [krew](https://krew.sigs.k8s.io/)

```shell
kubectl krew install lineage

kubectl lineage --version
```

### Install from Source

```shell
git clone git@github.com:tohjustin/kube-lineage.git && cd kube-lineage
make install

kube-lineage --version
```

## Prior Art

kube-lineage has been inspired by the following projects:

- [ahmetb/kubectl-tree](https://github.com/ahmetb/kubectl-tree)
- [feloy/kubectl-service-tree](https://github.com/feloy/kubectl-service-tree)
- [nimakaviani/knative-inspect](https://github.com/nimakaviani/knative-inspect/)
- [steveteuber/kubectl-graph](https://github.com/steveteuber/kubectl-graph)
