package samizdat

func AllSamizdat() []string {
	return currentState.listify(currentState.findRoot())
}
