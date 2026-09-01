package command

import "sort"

// Executors lists every executable node in a tree, in a stable order.
//
// A query over the shape rather than registry bookkeeping: whoever builds a
// bundle asks it what it just assembled, and whoever loads one asks it what
// it must be able to invoke.
func Executors(root Root) []ExecID {
	unique := make(map[ExecID]struct{})
	collectExecutors(root.Children, unique)
	executors := make([]ExecID, 0, len(unique))
	for executor := range unique {
		executors = append(executors, executor)
	}
	sort.Slice(executors, func(i, j int) bool { return executors[i] < executors[j] })
	return executors
}

func collectExecutors(nodes []Node, out map[ExecID]struct{}) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Literal:
			if typed.Exec != 0 {
				out[typed.Exec] = struct{}{}
			}
			collectExecutors(typed.Children, out)
		case Argument:
			if typed.Exec != 0 {
				out[typed.Exec] = struct{}{}
			}
			collectExecutors(typed.Children, out)
		}
	}
}
