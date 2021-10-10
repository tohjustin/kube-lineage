package printers

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tohjustin/kube-lineage/internal/graph"
)

type Interface interface {
	Print(w io.Writer, nodeMap graph.NodeMap, rootUID types.UID, maxDepth uint) error
}

type tablePrinter struct {
	configFlags  *HumanPrintFlags
	outputFormat string
}

func (p *tablePrinter) Print(w io.Writer, nodeMap graph.NodeMap, rootUID types.UID, maxDepth uint) error {
	root, ok := nodeMap[rootUID]
	if !ok {
		return fmt.Errorf("requested object (uid: %s) not found in list of fetched objects", rootUID)
	}

	return p.printTable(w, nodeMap, root, maxDepth)
}

func (p *tablePrinter) printTable(w io.Writer, nodeMap graph.NodeMap, root *graph.Node, maxDepth uint) error {
	// Generate Table to print
	showGroup := false
	if sg := p.configFlags.ShowGroup; sg != nil {
		showGroup = *sg
	}
	showGroupFn := createShowGroupFn(nodeMap, showGroup, maxDepth)
	t, err := nodeMapToTable(nodeMap, root, maxDepth, showGroupFn)
	if err != nil {
		return err
	}

	// Setup Table printer
	p.configFlags.SetShowNamespace(shouldShowNamespace(nodeMap, maxDepth))
	tableprinter, err := p.configFlags.ToPrinter(p.outputFormat)
	if err != nil {
		return err
	}

	return tableprinter.PrintObj(t, w)
}
