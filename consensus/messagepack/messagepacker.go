package messagepack

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"

	"github.com/sasha-s/go-deadlock"
	"github.com/stackerstan/go-nostr"
	"mindmachine/database"

	"mindmachine/mindmachine"
)

func PackMessage(message interface{}) {
	if event, ok := message.(mindmachine.Event); ok {
		packEvent(event.Nostr())
	}
	if event, ok := message.(nostr.Event); ok {
		packEvent(event)
	}
}

func marshal(p *messagepack) []byte {
	//todo change this to json
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	_p := messagepack{
		Height:   p.Height,
		Messages: p.Messages,
	}
	err := enc.Encode(_p)
	if err != nil {
		mindmachine.LogCLI(err.Error(), 0)
	}
	return buf.Bytes()
}

func unmarshal(f *os.File, mp *messagepack) error {
	//todo change this to json
	dec := gob.NewDecoder(f)
	err := dec.Decode(mp)
	if err != nil {
		return err
	}
	mp.mutex = &deadlock.Mutex{}
	return nil
}

func GetMessagePacks(start int64) []string {
	bloom := mindmachine.MakeNewInverseBloomFilter(10000)
	end := current.Height
	var eventIds []string
	for s := start; s < end; s++ {
		mp, ok := getMessagePack(s)
		if ok {
			for _, id := range mp.Messages {
				if bloom(id) {
					eventIds = append(eventIds, id)
				} else {
					//fmt.Printf("\n67\n%s\n", id)
				}
			}
		}
	}
	if end == current.Height {
		for _, message := range current.Messages {
			eventIds = append(eventIds, message)
		}
		return eventIds
	}
	return GetMessagePacks(start)
}

func getMessagePack(height int64) (m messagepack, ok bool) {
	file, ok := database.Open("messagepack", fmt.Sprintf("%d", height))
	if !ok {
		return
	}
	defer file.Close()
	if err := unmarshal(file, &m); err != nil {
		mindmachine.LogCLI(err.Error(), 1)
		return
	}
	return m, true
}
