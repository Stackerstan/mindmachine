package blocks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/davecgh/go-spew/spew"

	"mindmachine/mindmachine"
)

var blockChannel = make(chan mindmachine.BlockHeader)

func InsertBlock(bh mindmachine.BlockHeader) {
	blockChannel <- bh
}

func SubscribeToBlocks() chan mindmachine.BlockHeader {
	//var blockChannel = make(chan bitswarm.BlockHeader)
	go func() {
		listenForNewBlocksFromPublicAPI(blockChannel)
	}()
	return blockChannel
}

// listenForNewBlocksFromPublicAPI checks the blockserver (blockchain.info) for new blocks and sends them to the provided channel.
// when first called, it will send our latest local block.
// todo: problem: blockchain.info proves nothing. Solution: use a full node instead of blockchain.info
// subscribe to blocks on relays from any pubkey we know has votepower
func listenForNewBlocksFromPublicAPI(listener chan mindmachine.BlockHeader) {
	//var latest mindmachine.BlockHeader
	var bitcoinTip mindmachine.BlockHeader
	// StartAndSubscribe looping and watching for a terminate signal or a new block.
	for {
		select {
		case <-time.After(time.Second * 20):
			response, err := fetchLatestBlockFromNetwork()
			if err != nil {
				mindmachine.LogCLI(err, 2)
			}
			if err == nil && response != bitcoinTip {
				bitcoinTip = response
				mindmachine.SetBitcoinTip(bitcoinTip)
				nextBlockHeight := mindmachine.CurrentState().Processing.Height + 1
				if response.Height+1 < nextBlockHeight {
					// This should not happen except during ignition
					report := fmt.Sprintf("our block depth is too shallow, the latest Bitcoin block is: %v, our latest is: %v", bitcoinTip.Height, mindmachine.CurrentState().Processing.Height)
					mindmachine.LogCLI(report, 4)
				}

				if bitcoinTip.Height == nextBlockHeight {
					//latest, err = FetchBlock(bitcoinTip.Height)
					//if err != nil {
					//	mindmachine.LogCLI(err.Error(), 0)
					//	return
					//}
					//header, err := FetchBlock(bitcoinTip.Height - 1)
					//if err != nil {
					//	mindmachine.LogCLI(err.Error(), 0)
					//	return
					//}
					fmt.Println(64)
					listener <- bitcoinTip
					fmt.Println(66)
				}
				//if bitcoinTip.Height > nextBlockHeight {
				//	h, err := FetchBlock(nextBlockHeight)
				//	if err != nil {
				//		mindmachine.LogCLI(err.Error(), 1)
				//		break
				//	}
				//	listener <- h
				//}
			}
		}
	}
}

// FetchLatestBlock returns the latest Bitcoin block header
func FetchLatestBlock() (mindmachine.BlockHeader, bool) {
	if latest, err := fetchLatestBlockFromNetwork(); err == nil {
		return latest, true
	}
	return mindmachine.BlockHeader{}, false
}

func fetchLatestBlockFromNetwork() (mindmachine.BlockHeader, error) {
	bh := mindmachine.BlockHeader{}
	client := &http.Client{}
	url := mindmachine.MakeOrGetConfig().GetString("blockServer") + "/latestblock"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return bh, err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		if mindmachine.MakeOrGetConfig().GetBool("devMode") {
		}
		return bh, err
	}
	if resp.StatusCode != 200 {
		time.Sleep(time.Second)
		mindmachine.LogCLI(resp.Request.URL.String(), 2)
		mindmachine.LogCLI(resp, 1)
		return fetchLatestBlockFromNetwork()
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		mindmachine.LogCLI(err, 1)
		time.Sleep(time.Second)
		return fetchLatestBlockFromNetwork()
	}
	err = json.Unmarshal(bodyBytes, &bh)
	if err != nil {
		return bh, err
	}
	err = resp.Body.Close()
	if err != nil {
		return bh, err
	}
	return bh, nil
}

func FetchBlock(h int64) (mindmachine.BlockHeader, error) {
	//fmt.Printf("\n120\n%d\n", h)
	//defer fmt.Printf("\n121\n%d\n", h)
	if h < 0 {
		return mindmachine.BlockHeader{}, fmt.Errorf("can't request a block height of less than 0")
	}
	client := &http.Client{}
	url := mindmachine.MakeOrGetConfig().GetString("blockServer") + "/rawblock/" + fmt.Sprint(h)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return mindmachine.BlockHeader{}, err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return mindmachine.BlockHeader{}, err
	}
	if resp.StatusCode != 200 {
		mindmachine.LogCLI(resp, 3)
		time.Sleep(time.Second)
		return FetchBlock(h)
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		mindmachine.LogCLI(err, 3)
		time.Sleep(time.Second)
		return FetchBlock(h)
	}
	var responseObject interface{}
	err = json.Unmarshal(bodyBytes, &responseObject)
	if err != nil {
		spew.Dump(bodyBytes)
		return mindmachine.BlockHeader{}, err
	}
	data := responseObject.(map[string]interface{})
	header := mindmachine.BlockHeader{
		Height: h,
	}
	for k, v := range data {
		if k == "hash" {
			header.Hash = v.(string)
		}
		if k == "time" {
			header.Time = int64(v.(float64))
		}
	}
	err = resp.Body.Close()
	if err != nil {
		return mindmachine.BlockHeader{}, err
	}
	return header, nil
}
