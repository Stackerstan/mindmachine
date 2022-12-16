package mindmachine

import (
	"fmt"

	"github.com/sasha-s/go-deadlock"
)

var validKinds = make(map[int64]string)
var kindsMutex = &deadlock.Mutex{}

func WhichMindForKind(kind int64) (string, bool) {
	kindsMutex.Lock()
	defer kindsMutex.Unlock()
	mind, ok := validKinds[kind]
	return mind, ok
}

func registerKinds(kinds []int64, mind string) error {
	kindsMutex.Lock()
	defer kindsMutex.Unlock()
	for _, kind := range kinds {
		_mind, ok := validKinds[kind]
		if ok {
			return fmt.Errorf("this Kind has already been registered by %s", _mind)
		}
		validKinds[kind] = mind
	}
	return nil
}

func GetAllKinds() map[int64]string {
	kindsMutex.Lock()
	defer kindsMutex.Unlock()
	return validKinds
}
