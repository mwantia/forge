package ui

// pluginNamespacesFrom returns the names of all non-builtin namespaces from l.
func pluginNamespacesFrom(l namespaceLister) []string {
	if l == nil {
		return nil
	}

	ns := l.ListNamespaces()
	names := make([]string, 0, len(ns))

	for _, n := range ns {
		if !n.Builtin {
			names = append(names, n.Namespace)
		}
	}
	
	return names
}
