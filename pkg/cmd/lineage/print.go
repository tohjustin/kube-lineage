package lineage

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredv1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/util/jsonpath"
)

const (
	cellUnknown = "<unknown>"
	cellUnset   = "<none>"
)

var (
	objectColumnDefinitions = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: metav1.ObjectMeta{}.SwaggerDoc()["name"]},
		{Name: "Status", Type: "string", Description: "The condition Ready status of the object."},
		{Name: "Reason", Type: "string", Description: "The condition Ready reason of the object."},
		{Name: "Age", Type: "string", Description: metav1.ObjectMeta{}.SwaggerDoc()["creationTimestamp"]},
	}
)

// printOptions defines various print options
type printOptions struct {
	NoHeaders     bool
	ShowLabels    bool
	WithNamespace bool
}

// objectColumns holds columns for all kinds of Kubernetes objects
type objectColumns struct {
	Name   string
	Status string
	Reason string
	Age    string
}

func printGraph(objects Graph, uid types.UID, options printOptions) error {
	// TODO: Sort graph before printing
	rows, err := printTableRows(objects, uid, "", options)
	if err != nil {
		return err
	}

	return tablePrinter(objectColumnDefinitions, rows, printers.PrintOptions{
		NoHeaders:     options.NoHeaders,
		WithNamespace: options.WithNamespace,
		ShowLabels:    options.ShowLabels,
	})
}

// TODO: Refactor this to remove duplication
func printTableRows(objects Graph, uid types.UID, prefix string, options printOptions) ([]metav1.TableRow, error) {
	var rows []metav1.TableRow
	node := objects[uid]

	if len(prefix) == 0 {
		columns := getObjectColumns(*node.Unstructured)
		row := metav1.TableRow{
			Object: runtime.RawExtension{
				Object: node.DeepCopyObject(),
			},
			Cells: []interface{}{
				columns.Name,
				columns.Status,
				columns.Reason,
				columns.Age,
			},
		}
		rows = append(rows, row)
	}

	for i, childUID := range node.Dependents {
		child := objects[childUID]

		// Compute prefix
		var rowPrefix, childPrefix string
		if i != len(node.Dependents)-1 {
			rowPrefix, childPrefix = prefix+"├── ", prefix+"│   "
		} else {
			rowPrefix, childPrefix = prefix+"└── ", prefix+"    "
		}

		columns := getObjectColumns(*child.Unstructured)
		row := metav1.TableRow{
			Object: runtime.RawExtension{
				Object: child.DeepCopyObject(),
			},
			Cells: []interface{}{
				rowPrefix + columns.Name,
				columns.Status,
				columns.Reason,
				columns.Age,
			},
		}
		rows = append(rows, row)

		childRows, err := printTableRows(objects, childUID, childPrefix, options)
		if err != nil {
			return nil, err
		}
		rows = append(rows, childRows...)
	}

	return rows, nil
}

func getNestedString(u unstructuredv1.Unstructured, name, jsonPath string) (string, error) {
	jp := jsonpath.New(name).AllowMissingKeys(true)
	if err := jp.Parse(jsonPath); err != nil {
		return "", err
	}

	data := u.UnstructuredContent()
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

func getObjectColumns(u unstructuredv1.Unstructured) *objectColumns {
	status, _ := getNestedString(u, "condition-ready-status", "{.status.conditions[?(@.type==\"Ready\")].status}")
	if len(status) == 0 {
		status = cellUnset
	}
	reason, _ := getNestedString(u, "condition-ready-reason", "{.status.conditions[?(@.type==\"Ready\")].reason}")
	if len(reason) == 0 {
		reason = cellUnset
	}

	return &objectColumns{
		Name:   fmt.Sprintf("%s/%s", u.GetKind(), u.GetName()),
		Status: status,
		Reason: reason,
		Age:    translateTimestampSince(u.GetCreationTimestamp()),
	}
}

// translateTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return cellUnknown
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}

// tablePrinter prints the table using the cli-runtime TablePrinter
func tablePrinter(columns []metav1.TableColumnDefinition, rows []metav1.TableRow, options printers.PrintOptions) error {
	table := &metav1.Table{
		ColumnDefinitions: columns,
		Rows:              rows,
	}
	out := bytes.NewBuffer([]byte{})
	printer := printers.NewTablePrinter(options)
	if err := printer.PrintObj(table, out); err != nil {
		return err
	}
	fmt.Printf("%s", out.String())

	return nil
}
