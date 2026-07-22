package fsst

// qsymHeap and selectCandidates preserve the immutable benchmark root's old
// call shape only in test builds. Production uses selectTopCandidates with one
// reusable scratch slice; this removable adapter delegates the benchmark to
// that production implementation without adding a production fallback.
type qsymHeap []qsym

func selectCandidates(candidates map[[2]uint64]qsym, heap *qsymHeap, list *[]qsym) {
	scratch := []qsym(*heap)
	*list = selectTopCandidates(candidates, &scratch)
	*heap = qsymHeap(scratch)
}
