package problems

import (
	"sort"
)

//GetAllProblemsInOrder retrieves all the Problems from the database and orders them with parents followed by children
func GetAllProblemsInOrder() (items []Problem) {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	unique := make(map[string]struct{})
	for _, item := range currentState.data {
		unique[item.findRoot().UID] = struct{}{}
	}
	var roots []Problem
	for s, _ := range unique {
		roots = append(roots, currentState.data[s])
	}
	roots = sortItems(roots)
	items = append(items, appendChildren(roots)...)
	for _, item := range items {
		for _, i2 := range item.getDirectChildren() {
			item.Children = append(item.Children, i2.UID)
		}
	}
	return
}

func appendChildren(items []Problem) (a []Problem) {
	for _, item := range items {
		a = append(a, item)
		c := item.getDirectChildren()
		if len(c) != 0 {
			c = sortItems(c)
			a = append(a, appendChildren(c)...)
		}
	}
	return a
}

func sortItems(items []Problem) []Problem {
	sort.Slice(items, func(i, j int) bool {
		return items[i].WitnessedAt > items[j].WitnessedAt
	})
	return items
}

func (item *Problem) findRoot() Problem {
	if len(item.Parent) == 64 {
		if p, ok := currentState.data[item.Parent]; ok {
			return p.findRoot()
		}
	}
	return *item
}
