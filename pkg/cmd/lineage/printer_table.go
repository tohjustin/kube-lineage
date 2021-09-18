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
)

const (
	cellUnknown = "<unknown>"
	cellUnset   = "<none>"
)

var (
	// objectColumnDefinitions holds table column definition for Kubernetes objects.
	objectColumnDefinitions = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: metav1.ObjectMeta{}.SwaggerDoc()["name"]},
		{Name: "Status", Type: "string", Description: "The condition Ready status of the object."},
		{Name: "Reason", Type: "string", Description: "The condition Ready reason of the object."},
		{Name: "Age", Type: "string", Description: metav1.ObjectMeta{}.SwaggerDoc()["creationTimestamp"]},
	}
	// objectReasonJSONPath is the JSON path to get a Kubernetes object's "Ready"
	// condition-type status.
	objectReasonJSONPath *jsonpath.JSONPath
	// objectStatusJSONPath is the JSON path to get a Kubernetes object's "Ready"
	// condition-type reason.
	objectStatusJSONPath *jsonpath.JSONPath
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

func init() {
	var err error
	objectReasonJSONPath, err = newJSONPath("status", "{.status.conditions[?(@.type==\"Ready\")].reason}")
	if err != nil {
		err = fmt.Errorf("failed to initialize object reason JSON path: %w", err)
		panic(err)
	}
	objectStatusJSONPath, err = newJSONPath("status", "{.status.conditions[?(@.type==\"Ready\")].status}")
	if err != nil {
		err = fmt.Errorf("failed to initialize object status JSON path: %w", err)
		panic(err)
	}
}

// newJSONPath returns a JSONPath object created from parsing the provided JSON
// path expression.
func newJSONPath(name, jsonPath string) (*jsonpath.JSONPath, error) {
	jp := jsonpath.New(name).AllowMissingKeys(true)
	if err := jp.Parse(jsonPath); err != nil {
		return nil, err
	}
	return jp, nil
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

// nodeToTableRow converts the provided node into a table row.
func nodeToTableRow(node *Node, namePrefix string, showGroupFn func(kind string) bool) metav1.TableRow {
	var name string
	if showGroupFn(node.Kind) && len(node.Group) > 0 {
		name = fmt.Sprintf("%s%s.%s/%s", namePrefix, node.Kind, node.Group, node.Name)
	} else {
		name = fmt.Sprintf("%s%s/%s", namePrefix, node.Kind, node.Name)
	}
	data := node.UnstructuredContent()
	status, _ := getNestedString(data, objectStatusJSONPath)
	if len(status) == 0 {
		status = cellUnset
	}
	reason, _ := getNestedString(data, objectReasonJSONPath)
	if len(reason) == 0 {
		reason = cellUnset
	}
	age := translateTimestampSince(node.GetCreationTimestamp())

	return metav1.TableRow{
		Object: runtime.RawExtension{Object: node.DeepCopyObject()},
		Cells: []interface{}{
			name,
			status,
			reason,
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
