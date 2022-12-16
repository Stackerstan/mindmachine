package problems

func (item *Problem) hasOpenChildren() bool {
	for _, i := range currentState.data {
		if i.Parent == item.UID {
			if i.Closed != true {
				return true
			}
		}
	}
	return false
}

func (item *Problem) getDirectChildren() (c []Problem) {
	for _, i := range currentState.data {
		if i.Parent == item.UID {
			c = append(c, i)
		}
	}
	return
}
