package mindmachine

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/nbd-wtf/go-nostr/nip06"
	"github.com/sasha-s/go-deadlock"
)

var currentWallet Wallet
var currentWalletMutex = &deadlock.Mutex{}

// MyWallet returns the current Wallet or creates a new one if there isn't one already
func MyWallet() Wallet {
	currentWalletMutex.Lock()
	defer currentWalletMutex.Unlock()
	if len(currentWallet.PrivateKey) == 0 {
		//try to restore wallet from disk
		if w, ok := getWalletFromDisk(); ok {
			currentWallet = w
		} else {
			LogCLI("Generating a new wallet, write down the seed words if you want to keep it", 4)
			currentWallet = makeNewWallet()
			fmt.Printf("\n\n~NEW WALLET~\nPublic Key: %s\nPrivate Key: %s\nSeed Words: %s\n\n", currentWallet.Account, currentWallet.PrivateKey, currentWallet.SeedWords)
		}
	}
	if err := persistCurrentWallet(); err != nil {
		LogCLI(err.Error(), 0)
	}
	return currentWallet
}

// ImportWallet takes a private key in WIF and stores the public address + private key.
// The address and key can be called with MyWallet()
//func ImportWallet(key string) error {
//	wallet, err := importWalletFromWIF(key)
//	if err != nil {
//		return err
//	}
//	currentWalletMutex.Lock()
//	currentWallet = wallet
//	currentWalletMutex.Unlock()
//	err = persistCurrentWallet()
//	if err != nil {
//		return err
//	}
//	return nil
//}

func makeNewWallet() Wallet {
	seedWords, err := nip06.GenerateSeedWords()
	if err != nil {
		LogCLI(err.Error(), 0)
	}
	seed := nip06.SeedFromWords(seedWords)
	sk, err := nip06.PrivateKeyFromSeed(seed)
	if err != nil {
		LogCLI(err.Error(), 0)
	}
	return Wallet{
		PrivateKey: sk,
		SeedWords:  seedWords,
		Account:    getPubKey(sk),
	}
}

func getPubKey(privateKey string) string {
	if keyb, err := hex.DecodeString(privateKey); err != nil {
		LogCLI(fmt.Sprintf("Error decoding key from hex: %s\n", err.Error()), 0)
	} else {
		_, pubkey := btcec.PrivKeyFromBytes(keyb)
		return hex.EncodeToString(pubkey.X().Bytes())
	}
	return ""
}

//func createNewWallet() error {
//	privateKey, _ := createPrivateKey()
//	address, _ := getAddress(privateKey)
//	currentWalletMutex.Lock()
//	currentWallet = Wallet{
//		PrivateKey: privateKey.String(),
//		Account:    address.EncodeAddress(),
//	}
//	currentWalletMutex.Unlock()
//	err := persistCurrentWallet()
//	if err != nil {
//		return err
//	}
//	return nil
//}

//func importWalletFromWIF(wifStr string) (Wallet, error) {
//	wif, err := btcutil.DecodeWIF(wifStr)
//	if err != nil {
//		return Wallet{}, err
//	}
//	if !wif.IsForNet(getNetworkParams()) {
//		return Wallet{}, errors.New("the WIF string is not valid")
//	}
//	address, err := getAddress(wif)
//	if err != nil {
//		return Wallet{}, err
//	}
//	wallet := Wallet{
//		PrivateKey: wif.String(),
//		Account:    address.EncodeAddress(),
//	}
//	return wallet, nil
//}

//func getNetworkParams() *chaincfg.Params {
//	networkParams := &chaincfg.MainNetParams
//	networkParams.PubKeyHashAddrID = 0x00
//	networkParams.PrivateKeyID = 0x80
//	return networkParams
//}

//func createPrivateKey() (*btcutil.WIF, error) {
//	secret, err := btcec.NewPrivateKey(btcec.S256())
//	if err != nil {
//		return nil, err
//	}
//	return btcutil.NewWIF(secret, getNetworkParams(), true)
//}
//
//func getAddress(wif *btcutil.WIF) (*btcutil.AddressPubKey, error) {
//	return btcutil.NewAddressPubKey(wif.PrivKey.PubKey().SerializeCompressed(), getNetworkParams())
//}

func persistCurrentWallet() error {
	file, err := os.Create(MakeOrGetConfig().GetString("rootDir") + "wallet.dat")
	if err != nil {
		LogCLI(err.Error(), 0)
	}
	defer file.Close()
	bytes, err := json.Marshal(currentWallet)
	if err != nil {
		LogCLI(err.Error(), 0)
	}
	_, err = file.Write(bytes)
	if err != nil {
		LogCLI(err.Error(), 0)
	}
	return nil
}

func getWalletFromDisk() (w Wallet, ok bool) {
	file, err := ioutil.ReadFile(MakeOrGetConfig().GetString("rootDir") + "wallet.dat")
	if err != nil {
		LogCLI(fmt.Sprintf("Error getting wallet file: %s", err.Error()), 2)
		return Wallet{}, false
	}
	err = json.Unmarshal(file, &w)
	if err != nil {
		LogCLI(fmt.Sprintf("Error parsing wallet file: %s", err.Error()), 3)
		return Wallet{}, false
	}
	return w, true
}
