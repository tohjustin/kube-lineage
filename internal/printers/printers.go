package printers

import (
	"context"
	"fmt"
	"io"
	"sort"

	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/tohjustin/kube-lineage/internal/client"
	"github.com/tohjustin/kube-lineage/internal/graph"
)

type sortableGroupKind []schema.GroupKind

func (s sortableGroupKind) Len() int           { return len(s) }
func (s sortableGroupKind) Less(i, j int) bool { return lessGroupKind(s[i], s[j]) }
func (s sortableGroupKind) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func lessGroupKind(lhs, rhs schema.GroupKind) bool {
	return lhs.String() < rhs.String()
}

type Interface interface {
	Print(w io.Writer, nodeMap graph.NodeMap, rootUID types.UID, maxDepth uint, depsIsDependencies bool) error
}

type tablePrinter struct {
	configFlags  *HumanPrintFlags
	outputFormat string

	// client for fetching server-printed tables when printing in split output
	// format
	client client.Interface
}

func (p *tablePrinter) Print(w io.Writer, nodeMap graph.NodeMap, rootUID types.UID, maxDepth uint, depsIsDependencies bool) error {
	root, ok := nodeMap[rootUID]
	if !ok {
		return fmt.Errorf("requested object (uid: %s) not found in list of fetched objects", rootUID)
	}

	if p.configFlags.IsSplitOutputFormat(p.outputFormat) {
		if p.client == nil {
			return fmt.Errorf("client must be provided to get server-printed tables")
		}
		return p.printTablesByGK(w, nodeMap, maxDepth)
	}

	return p.printTable(w, nodeMap, root, maxDepth, depsIsDependencies)
}

func (p *tablePrinter) printTable(w io.Writer, nodeMap graph.NodeMap, root *graph.Node, maxDepth uint, depsIsDependencies bool) error {
	// Generate Table to print
	showGroup := false
	if sg := p.configFlags.ShowGroup; sg != nil {
		showGroup = *sg
	}
	showGroupFn := createShowGroupFn(nodeMap, showGroup, maxDepth)
	t, err := nodeMapToTable(nodeMap, root, maxDepth, depsIsDependencies, showGroupFn)
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

func (p *tablePrinter) printTablesByGK(w io.Writer, nodeMap graph.NodeMap, maxDepth uint) error {
	// Generate Tables to print
	showGroup, showNamespace := false, false
	if sg := p.configFlags.ShowGroup; sg != nil {
		showGroup = *sg
	}
	if sg := p.configFlags.ShowNamespace; sg != nil {
		showNamespace = *sg
	}
	showGroupFn := createShowGroupFn(nodeMap, showGroup, maxDepth)
	showNamespaceFn := createShowNamespaceFn(nodeMap, showNamespace, maxDepth)

	tListByGK, err := p.nodeMapToTableByGK(nodeMap, maxDepth)
	if err != nil {
		return err
	}

	// Sort Tables by GroupKind
	var gkList sortableGroupKind
	for gk := range tListByGK {
		gkList = append(gkList, gk)
	}
	sort.Sort(gkList)
	for ix, gk := range gkList {
		if t, ok := tListByGK[gk]; ok {
			// Setup Table printer
			tgk := gk
			if !showGroupFn(gk.Kind) {
				tgk = schema.GroupKind{Kind: gk.Kind}
			}
			p.configFlags.SetShowNamespace(showNamespaceFn(gk))
			tableprinter, err := p.configFlags.ToPrinterWithGK(p.outputFormat, tgk)
			if err != nil {
				return err
			}

			// Setup Table printer
			err = tableprinter.PrintObj(t, w)
			if err != nil {
				return err
			}
			if ix != len(gkList)-1 {
				fmt.Fprintf(w, "\n")
			}
		}
	}

	return nil
}

//nolint:funlen,gocognit
func (p *tablePrinter) nodeMapToTableByGK(nodeMap graph.NodeMap, maxDepth uint) (map[schema.GroupKind](*metav1.Table), error) {
	// Filter objects to print based on depth
	objUIDs := []types.UID{}
	for uid, node := range nodeMap {
		if maxDepth == 0 || node.Depth <= maxDepth {
			objUIDs = append(objUIDs, uid)
		}
	}

	// Group objects by GroupKind & Namespace
	nodesByGKAndNS := map[schema.GroupKind](map[string]graph.NodeList){}
	for _, uid := range objUIDs {
		if node, ok := nodeMap[uid]; ok {
			gk := schema.GroupKind{Group: node.Group, Kind: node.Kind}
			ns := node.Namespace
			if _, ok := nodesByGKAndNS[gk]; !ok {
				nodesByGKAndNS[gk] = map[string]graph.NodeList{}
			}
			nodesByGKAndNS[gk][ns] = append(nodesByGKAndNS[gk][ns], node)
		}
	}

	// Fan-out to get server-print tables for all objects
	eg, ctx := errgroup.WithContext(context.Background())
	tableByGKAndNS := map[schema.GroupKind](map[string]*metav1.Table){}
	for gk, nodesByNS := range nodesByGKAndNS {
		if len(gk.Kind) == 0 {
			continue
		}
		for ns, nodes := range nodesByNS {
			if len(nodes) == 0 {
				continue
			}
			gk, api, ns, names := gk, client.APIResource(nodes[0].GetAPIResource()), ns, []string{}
			for _, n := range nodes {
				names = append(names, n.Name)
			}
			// Sort TableRows by name
			sortedNames := sets.NewString(names...).List()
			eg.Go(func() error {
				table, err := p.client.GetTable(ctx, client.GetTableOptions{
					APIResource: api,
					Namespace:   ns,
					Names:       sortedNames,
				})
				if err != nil || table == nil {
					return err
				}
				if _, ok := tableByGKAndNS[gk]; !ok {
					tableByGKAndNS[gk] = map[string]*metav1.Table{}
				}
				if t, ok := tableByGKAndNS[gk][ns]; !ok {
					tableByGKAndNS[gk][ns] = table
				} else {
					t.Rows = append(t.Rows, table.Rows...)
				}
				return nil
			})
		}
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// Sort TableRows by namespace
	tableByGK := map[schema.GroupKind]*metav1.Table{}
	for gk, tableByNS := range tableByGKAndNS {
		var nsList []string
		for ns := range tableByNS {
			nsList = append(nsList, ns)
		}
		sortedNSList := sets.NewString(nsList...).List()
		var table *metav1.Table
		for _, ns := range sortedNSList {
			if t, ok := tableByNS[ns]; ok {
				if table == nil {
					table = t
				} else {
					table.Rows = append(table.Rows, t.Rows...)
				}
			}
		}
		tableByGK[gk] = table
	}

	return tableByGK, nil
}
