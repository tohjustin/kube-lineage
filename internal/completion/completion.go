package completion

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// filterString returns all strings from 's', except those with names matching
// 'ignored'.
func filterString(s []string, ignored []string) []string {
	if ignored == nil {
		return s
	}
	var filteredStrList []string
	for _, str := range s {
		found := false
		for _, ignoredName := range ignored {
			if str == ignoredName {
				found = true
				break
			}
		}
		if !found {
			filteredStrList = append(filteredStrList, str)
		}
	}
	return filteredStrList
}

// GetScopeNamespaceList provides dynamic auto-completion for scope namespaces.
func GetScopeNamespaceList(f cmdutil.Factory, cmd *cobra.Command, toComplete string) []string {
	var comp []string

	allNS := get.CompGetResource(f, cmd, "namespace", "")
	existingNS := strings.Split(toComplete, ",")
	existingNS = existingNS[:len(existingNS)-1]
	ignoreNS := existingNS
	if ns, _, err := f.ToRawKubeConfigLoader().Namespace(); err == nil {
		ignoreNS = append(ignoreNS, ns)
	}
	filteredNS := filterString(allNS, ignoreNS)

	compPrefix := strings.Join(existingNS, ",")
	for _, ns := range filteredNS {
		if len(compPrefix) > 0 {
			ns = fmt.Sprintf("%s,%s", compPrefix, ns)
		}
		comp = append(comp, ns)
	}

	return comp
}
