package protocol

import (
	"mindmachine/mindmachine"
)

func GetFullProtocol() (e []Item) {
	items := currentState.copyOfCurrent()
	//15171031
	root := items["1a54f1f4ceabd11ef562cbb031f4ed0faf4091606fd9998b574dfb1b887e8b5d"]
	if root, ok := items[root.latest(items)]; ok {
		e = append(e, root)
		e = append(e, root.getNestedItems(items)...)
	}
	return
}

func (i *Item) getNestedItems(items map[mindmachine.S256Hash]Item) []Item {
	var list []Item
	if i.ApprovedAt > 0 {
		if latest, ok := items[i.latest(items)]; ok {
			for _, nestedID := range latest.Nests {
				if nestedItem, ok := items[nestedID]; ok {
					if latestNestedItem, ok := items[nestedItem.latest(items)]; ok {
						list = append(list, latestNestedItem)
						list = append(list, latestNestedItem.getNestedItems(items)...)
					}
				}
			}
		}
	}
	return list
}

//latest checks if the Item has been superseded and returns the current version if so
func (i *Item) latest(items map[mindmachine.S256Hash]Item) mindmachine.S256Hash {
	latest := i.UID
	for {
		if len(items[latest].SupersededBy) > 0 {
			latest = items[latest].SupersededBy
		} else {
			return latest
		}
	}
}