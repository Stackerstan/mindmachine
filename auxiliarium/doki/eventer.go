package doki

func GetAll() (d []Document) {
	currentState.mutex.Lock()
	defer currentState.mutex.Unlock()
	for _, document := range currentState.data {
		d = append(d, document)
	}
	return
}
