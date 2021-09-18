package lineage

import (
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/client-go/util/jsonpath"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/util/node"
)

const (
	cellUnknown       = "<unknown>"
	cellNotApplicable = "-"
)

var (
	// objectColumnDefinitions holds table column definition for Kubernetes objects.
	objectColumnDefinitions = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: metav1.ObjectMeta{}.SwaggerDoc()["name"]},
		{Name: "Ready", Type: "string", Description: "The readiness state of this object."},
		{Name: "Status", Type: "string", Description: "The status of this object."},
		{Name: "Age", Type: "string", Description: metav1.ObjectMeta{}.SwaggerDoc()["creationTimestamp"]},
	}
	// objectReadyReasonJSONPath is the JSON path to get a Kubernetes object's
	// "Ready" condition reason.
	objectReadyReasonJSONPath = newJSONPath("status", "{.status.conditions[?(@.type==\"Ready\")].reason}")
	// objectReadyStatusJSONPath is the JSON path to get a Kubernetes object's
	// "Ready" condition status.
	objectReadyStatusJSONPath = newJSONPath("status", "{.status.conditions[?(@.type==\"Ready\")].status}")
)

// NodeList represents an owner-dependent relationship tree stored as flat list
// of nodes.
type NodeList []*Node

func (n NodeList) Len() int {
	return len(n)
}

func (n NodeList) Less(i, j int) bool {
	// Sort nodes in following order: Namespace, Kind, Group, Name
	a, b := n[i], n[j]
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.Group != b.Group {
		return a.Group < b.Group
	}
	return a.Name < b.Name
}

func (n NodeList) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// newJSONPath returns a JSONPath object created from parsing the provided JSON
// path expression.
func newJSONPath(name, jsonPath string) *jsonpath.JSONPath {
	jp := jsonpath.New(name).AllowMissingKeys(true)
	if err := jp.Parse(jsonPath); err != nil {
		panic(err)
	}
	return jp
}

// getNestedString returns the field value of a Kubernetes object at the
// provided JSON path.
func getNestedString(data map[string]interface{}, jp *jsonpath.JSONPath) (string, error) {
	values, err := jp.FindResults(data)
	if err != nil {
		return "", err
	}
	strValues := []string{}
	for arrIx := range values {
		for valIx := range values[arrIx] {
			strValues = append(strValues, fmt.Sprintf("%v", values[arrIx][valIx].Interface()))
		}
	}
	str := strings.Join(strValues, ",")

	return str, nil
}

// getObjectReadyStatus returns the ready & status value of a Kubernetes object.
func getObjectReadyStatus(u *unstructuredv1.Unstructured) (string, string, error) {
	data := u.UnstructuredContent()
	ready, err := getNestedString(data, objectReadyStatusJSONPath)
	if err != nil {
		return "", "", err
	}
	status, err := getNestedString(data, objectReadyReasonJSONPath)
	if err != nil {
		return ready, "", err
	}

	return ready, status, nil
}

// getDaemonSetReadyStatus returns the ready & status value of a DaemonSet
// which is based off the table cell values computed by printDaemonSet from
// https://github.com/kubernetes/kubernetes/blob/v1.22.1/pkg/printers/internalversion/printers.go.
func getDaemonSetReadyStatus(u *unstructuredv1.Unstructured) (ready string, status string, err error) {
	var ds apps.DaemonSet
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &ds)
	if err != nil {
		return
	}

	desiredReplicas := ds.Status.DesiredNumberScheduled
	readyReplicas := ds.Status.NumberReady
	ready = fmt.Sprintf("%d/%d", int64(readyReplicas), int64(desiredReplicas))
	return
}

// getDeploymentReadyStatus returns the ready & status value of a Deployment
// which is based off the table cell values computed by printDeployment from
// https://github.com/kubernetes/kubernetes/blob/v1.22.1/pkg/printers/internalversion/printers.go.
func getDeploymentReadyStatus(u *unstructuredv1.Unstructured) (ready string, status string, err error) {
	var deploy apps.Deployment
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &deploy)
	if err != nil {
		return
	}

	desiredReplicas := deploy.Spec.Replicas
	readyReplicas := deploy.Status.ReadyReplicas
	ready = fmt.Sprintf("%d/%d", int64(readyReplicas), int64(desiredReplicas))
	return
}

// getPodReadyStatus returns the ready & status value of a Pod which is based
// off the table cell values computed by printPod from
// https://github.com/kubernetes/kubernetes/blob/v1.22.1/pkg/printers/internalversion/printers.go.
func getPodReadyStatus(u *unstructuredv1.Unstructured) (ready string, status string, err error) {
	var pod core.Pod
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &pod)
	if err != nil {
		return
	}

	restarts := 0
	totalContainers := len(pod.Spec.Containers)
	readyContainers := 0
	lastRestartDate := metav1.NewTime(time.Time{})

	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	initializing := false
	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]
		restarts += int(container.RestartCount)
		if container.LastTerminationState.Terminated != nil {
			terminatedDate := container.LastTerminationState.Terminated.FinishedAt
			if lastRestartDate.Before(&terminatedDate) {
				lastRestartDate = terminatedDate
			}
		}
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}
	if !initializing {
		restarts = 0
		hasRunning := false
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

			restarts += int(container.RestartCount)
			if container.LastTerminationState.Terminated != nil {
				terminatedDate := container.LastTerminationState.Terminated.FinishedAt
				if lastRestartDate.Before(&terminatedDate) {
					lastRestartDate = terminatedDate
				}
			}
			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				reason = container.State.Waiting.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				reason = container.State.Terminated.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else if container.Ready && container.State.Running != nil {
				hasRunning = true
				readyContainers++
			}
		}

		// change pod status back to "Running" if there is at least one container still reporting as "Running" status
		if reason == "Completed" && hasRunning {
			reason = "NotReady"
			for _, condition := range pod.Status.Conditions {
				if condition.Type == core.PodReady && condition.Status == core.ConditionTrue {
					reason = "Running"
				}
			}
		}
	}

	if pod.DeletionTimestamp != nil && pod.Status.Reason == node.NodeUnreachablePodReason {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	return fmt.Sprintf("%d/%d", readyContainers, totalContainers), reason, nil
}

// getReplicaSetReadyStatus returns the ready & status value of a ReplicaSet
// which is based off the table cell values computed by printReplicaSet from
// https://github.com/kubernetes/kubernetes/blob/v1.22.1/pkg/printers/internalversion/printers.go.
func getReplicaSetReadyStatus(u *unstructuredv1.Unstructured) (ready string, status string, err error) {
	var rs apps.ReplicaSet
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &rs)
	if err != nil {
		return
	}

	desiredReplicas := rs.Spec.Replicas
	readyReplicas := rs.Status.ReadyReplicas
	ready = fmt.Sprintf("%d/%d", int64(readyReplicas), int64(desiredReplicas))
	return
}

// getReplicationControllerReadyStatus returns the ready & status value of a
// ReplicationController which is based off the table cell values computed by
// printReplicationController from
// https://github.com/kubernetes/kubernetes/blob/v1.22.1/pkg/printers/internalversion/printers.go.
func getReplicationControllerReadyStatus(u *unstructuredv1.Unstructured) (ready string, status string, err error) {
	var rc core.ReplicationController
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &rc)
	if err != nil {
		return
	}

	desiredReplicas := rc.Spec.Replicas
	readyReplicas := rc.Status.ReadyReplicas
	ready = fmt.Sprintf("%d/%d", int64(readyReplicas), int64(desiredReplicas))
	return
}

// getStatefulSetReadyStatus returns the ready & status value of a StatefulSet
// which is based off the table cell values computed by printStatefulSet from
// https://github.com/kubernetes/kubernetes/blob/v1.22.1/pkg/printers/internalversion/printers.go.
func getStatefulSetReadyStatus(u *unstructuredv1.Unstructured) (ready string, status string, err error) {
	var sts apps.StatefulSet
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &sts)
	if err != nil {
		return
	}

	desiredReplicas := sts.Spec.Replicas
	readyReplicas := sts.Status.ReadyReplicas
	ready = fmt.Sprintf("%d/%d", int64(readyReplicas), int64(desiredReplicas))
	return
}

// nodeToTableRow converts the provided node into a table row.
func nodeToTableRow(node *Node, namePrefix string, showGroupFn func(kind string) bool) metav1.TableRow {
	var name, ready, status, age string

	if showGroupFn(node.Kind) && len(node.Group) > 0 {
		name = fmt.Sprintf("%s%s.%s/%s", namePrefix, node.Kind, node.Group, node.Name)
	} else {
		name = fmt.Sprintf("%s%s/%s", namePrefix, node.Kind, node.Name)
	}
	switch {
	case node.Group == "" && node.Kind == "Pod":
		ready, status, _ = getPodReadyStatus(node.Unstructured)
	case node.Group == "" && node.Kind == "ReplicationController":
		ready, status, _ = getReplicationControllerReadyStatus(node.Unstructured)
	case node.Group == "apps" && node.Kind == "DaemonSet":
		ready, status, _ = getDaemonSetReadyStatus(node.Unstructured)
	case node.Group == "apps" && node.Kind == "Deployment":
		ready, status, _ = getDeploymentReadyStatus(node.Unstructured)
	case node.Group == "apps" && node.Kind == "ReplicaSet":
		ready, status, _ = getReplicaSetReadyStatus(node.Unstructured)
	case node.Group == "apps" && node.Kind == "StatefulSet":
		ready, status, _ = getStatefulSetReadyStatus(node.Unstructured)
	default:
		ready, status, _ = getObjectReadyStatus(node.Unstructured)
	}
	if len(ready) == 0 {
		ready = cellNotApplicable
	}
	age = translateTimestampSince(node.GetCreationTimestamp())

	return metav1.TableRow{
		Object: runtime.RawExtension{Object: node.DeepCopyObject()},
		Cells: []interface{}{
			name,
			ready,
			status,
			age,
		},
	}
}

// printNode converts the provided node & its dependents into table rows.
func printNode(nodeMap NodeMap, root *Node, withGroup bool) ([]metav1.TableRow, error) {
	// Track every object kind in the node map & the groups that they belong to.
	kindToGroupSetMap := map[string](map[string]struct{}){}
	for _, node := range nodeMap {
		if _, ok := kindToGroupSetMap[node.Kind]; !ok {
			kindToGroupSetMap[node.Kind] = map[string]struct{}{}
		}
		kindToGroupSetMap[node.Kind][node.Group] = struct{}{}
	}
	// When printing an object & if there exists another object in the node map
	// that has the same kind but belongs to a different group (eg. "services.v1"
	// vs "services.v1.serving.knative.dev"), we prepend the object's name with
	// its GroupKind instead of its Kind to clearly indicate which resource type
	// it belongs to.
	showGroupFn := func(kind string) bool {
		return len(kindToGroupSetMap[kind]) > 1 || withGroup
	}
	// Sorts the list of UIDs based on the underlying object in following order:
	// Namespace, Kind, Group, Name
	sortUIDsFn := func(uids []types.UID) []types.UID {
		nodes := make(NodeList, len(uids))
		for ix, uid := range uids {
			nodes[ix] = nodeMap[uid]
		}
		sort.Sort(nodes)
		sortedUIDs := make([]types.UID, len(uids))
		for ix, node := range nodes {
			sortedUIDs[ix] = node.UID
		}
		return sortedUIDs
	}

	var rows []metav1.TableRow
	row := nodeToTableRow(root, "", showGroupFn)
	uidSet := map[types.UID]struct{}{}
	dependentRows, err := printNodeDependents(nodeMap, uidSet, root, "", sortUIDsFn, showGroupFn)
	if err != nil {
		return nil, err
	}
	rows = append(rows, row)
	rows = append(rows, dependentRows...)

	return rows, nil
}

// printNodeDependents converts the provided node's dependents into table rows.
func printNodeDependents(
	nodeMap NodeMap,
	uidSet map[types.UID]struct{},
	node *Node,
	prefix string,
	sortUIDsFn func(uids []types.UID) []types.UID,
	showGroupFn func(kind string) bool) ([]metav1.TableRow, error) {
	var rows []metav1.TableRow

	// Guard against possible cyclic dependency
	if _, ok := uidSet[node.UID]; ok {
		return rows, nil
	} else {
		uidSet[node.UID] = struct{}{}
	}

	dependents := sortUIDsFn(node.Dependents)
	lastIx := len(dependents) - 1
	for ix, childUID := range dependents {
		var childPrefix, dependentPrefix string
		if ix != lastIx {
			childPrefix, dependentPrefix = prefix+"├── ", prefix+"│   "
		} else {
			childPrefix, dependentPrefix = prefix+"└── ", prefix+"    "
		}

		child, ok := nodeMap[childUID]
		if !ok {
			return nil, fmt.Errorf("Dependent object (uid: %s) not found in list of fetched objects", childUID)
		}
		row := nodeToTableRow(child, childPrefix, showGroupFn)
		dependentRows, err := printNodeDependents(nodeMap, uidSet, child, dependentPrefix, sortUIDsFn, showGroupFn)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
		rows = append(rows, dependentRows...)
	}

	return rows, nil
}

// translateTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return cellUnknown
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}
