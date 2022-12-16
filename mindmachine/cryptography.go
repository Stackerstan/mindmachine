package mindmachine

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	boom "github.com/tylertreat/BoomFilters"
)

// Sign signs a message with using the standard Bitcoin message signing scheme
// and returns the signature as a base64 encoded string.
//func Sign(message []byte, privateKey PrivateKey) (string, error) {
//	// Decode private key from base58 wif format
//	wifkey, err := btcutil.DecodeWIF(fmt.Sprint(privateKey))
//	if err != nil {
//		return "", err
//	}
//
//	// create BIP322 message signing input
//	var buf bytes.Buffer
//	wire.WriteVarString(&buf, 0, "Bitcoin Signed Message:\n")
//	wire.WriteVarString(&buf, 0, string(message))
//
//	// make signature
//	//signature, err := btcec.SignCompact(btcec.S256(), wifkey.PrivKey, chainhash.DoubleHashB(buf.Bytes()), true)
//
//	if err != nil {
//		return "", err
//	}
//	return base64.StdEncoding.EncodeToString(signature), nil
//}

func Sign(message []byte, privateKey string) (signature string, e error) {
	hash := sha256.Sum256(message)

	s, err := hex.DecodeString(privateKey)
	if err != nil {
		return signature, fmt.Errorf("Sign called with invalid private key '%s': %w", privateKey, err)
	}
	sk, _ := btcec.PrivKeyFromBytes(s)

	sig, err := schnorr.Sign(sk, hash[:])
	if err != nil {
		return signature, err
	}

	return hex.EncodeToString(sig.Serialize()), nil
}

// ValidateSignedHash takes signature in base64 format and message and generates standard recovery of signature
// generating public key that must correspond to address (uncompressed address)
func ValidateSignedHash(msg S256Hash, signature string, account Account) (bool, error) {
	//message := fmt.Sprintf("%s", []byte(msg))
	//// create BIP322 message input
	//var buf bytes.Buffer
	//wire.WriteVarString(&buf, 0, "Bitcoin Signed Message:\n")
	//wire.WriteVarString(&buf, 0, message)
	//
	//// decode base64 encoded signature
	//signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	//if err != nil {
	//	return false, err
	//}
	//
	//// recover public key from signature
	//pub, wasCompressed, err := btcec.RecoverCompact(btcec.S256(), signatureBytes, chainhash.DoubleHashB(buf.Bytes()))
	//if err != nil {
	//	return false, err
	//}
	//
	//// derive the address for this public key
	//if wasCompressed {
	//	pk, err := btcutil.NewAddressPubKey(pub.SerializeCompressed(), &chaincfg.MainNetParams)
	//	if err != nil {
	//		return false, err
	//	}
	//	return pk.EncodeAddress() == account, nil
	//} else {
	//	pk, err := btcutil.NewAddressPubKey(pub.SerializeUncompressed(), &chaincfg.MainNetParams)
	//	if err != nil {
	//		return false, err
	//	}
	//	return pk.EncodeAddress() == account, nil
	//}
	return false, fmt.Errorf("not implemented")
}

func (raw *RawMessage) Sign(bh BlockHeader) {
	raw.From = MyWallet().Account
	raw.CreatedAt = bh
	raw.Time = time.Now().Unix()
	sig, err := Sign([]byte(fmt.Sprint(raw.Instructions, raw.CreatedAt, raw.Time)), MyWallet().PrivateKey)
	if err != nil {
		LogCLI(err.Error(), 0)
	}
	raw.Signature = sig
	return
}

func (raw *RawMessage) Verify() bool {
	hash := sha256.Sum256([]byte(fmt.Sprint(raw.Instructions, raw.CreatedAt, raw.Time)))
	// read and check pubkey
	pk, err := hex.DecodeString(raw.From)
	if err != nil {
		LogCLI(err.Error(), 1)
		return false
	}

	pubkey, err := schnorr.ParsePubKey(pk)
	if err != nil {
		LogCLI(err.Error(), 1)
		return false
	}

	// read signature
	s, err := hex.DecodeString(raw.Signature)
	if err != nil {
		LogCLI(err.Error(), 1)
		return false
	}
	sig, err := schnorr.ParseSignature(s)
	if err != nil {
		LogCLI(err.Error(), 1)
		return false
	}

	// check signature
	return sig.Verify(hash[:], pubkey)
}

//func validateRawMessage(m RawMessage) bool {
//	message := fmt.Sprintf("%s", []byte(fmt.Sprint(m.Instructions, m.CreatedAt, m.Time)))
//	// create standard Bitcoin signed message input
//	var buf bytes.Buffer
//	wire.WriteVarString(&buf, 0, "Bitcoin Signed Message:\n")
//	wire.WriteVarString(&buf, 0, message)
//
//	// decode base64 encoded signature
//	signatureBytes, err := base64.StdEncoding.DecodeString(m.Signature)
//	if err != nil {
//		LogCLI(err, 1)
//		return false
//	}
//
//	// recover pub key from signature
//	pub, wasCompressed, err := btcec.RecoverCompact(btcec.S256(), signatureBytes, chainhash.DoubleHashB(buf.Bytes()))
//	if err != nil {
//		fmt.Println(err)
//		return false
//	}
//
//	// generate corresponding address to public key
//	if wasCompressed {
//		pk, err := btcutil.NewAddressPubKey(pub.SerializeCompressed(), &chaincfg.MainNetParams)
//		if err != nil {
//			fmt.Println(err)
//			return false
//		}
//		return pk.EncodeAddress() == m.From
//	} else {
//		pk, err := btcutil.NewAddressPubKey(pub.SerializeUncompressed(), &chaincfg.MainNetParams)
//		if err != nil {
//			fmt.Println(err)
//			return false
//		}
//		return pk.EncodeAddress() == m.From
//	}
//}

func Sha256(data interface{}) S256Hash {
	var b []byte
	switch d := data.(type) {
	case string:
		b = []byte(d)
	case []byte:
		b = d
	default:
		LogCLI("attempted to hash non-string or non-[]byte", 0)
	}
	h := sha256.New()
	h.Write(b)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func MakeNewInverseBloomFilter(capacity uint) func(message interface{}) bool {
	ibf := boom.NewInverseBloomFilter(capacity)
	return func(message interface{}) bool {
		b := []byte(fmt.Sprint(message))
		// fmt.Printf("%s", b)
		return !ibf.TestAndAdd(b)
	}
}

func BloomCounter() func(raw RawMessage) int {
	blooms := make(map[int]func(message interface{}) bool)
	for i := 0; i < 10; i++ {
		blooms[i] = MakeNewInverseBloomFilter(10000)
	}
	return func(raw RawMessage) int {
		timesSeen := 0
		for i := 0; i < 10; i++ {
			if blooms[i](raw) {
				//this message has not been seen i times
				break
			}
			timesSeen = i
		}
		return timesSeen
	}
}

//AppendData adds the provided data to a buffer that lives as long as the HashSeq.
//Call HashSeq.S256 to hash the buffer and write the hash to HashSeq.Hash
func (h *HashSeq) AppendData(data interface{}) error {
	var errors []error
	switch d := data.(type) {
	case string:
		_, err := h.Data.WriteString(d)
		errors = append(errors, err)
	case int64:
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(d))
		_, err := h.Data.Write(b)
		errors = append(errors, err)
	case []byte:
		_, err := h.Data.Write(d)
		errors = append(errors, err)
	case []string:
		for _, s := range d {
			_, err := h.Data.WriteString(s)
			errors = append(errors, err)
		}
	case bool:
		if d {
			err := h.Data.WriteByte(1)
			errors = append(errors, err)
		}
		if !d {
			err := h.Data.WriteByte(0)
			errors = append(errors, err)
		}
	}
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

// S256 calculates the sha256 hash of the HashSeq and stores it as the HashSeq.Hash
//It resets the HashSeq.Data buffer.
func (h *HashSeq) S256() {
	h.Hash = fmt.Sprintf("%x", sha256.Sum256(h.Data.Bytes()))
	h.Data = bytes.Buffer{}
}

func Merkle(current [][]byte) (next [][]byte) {
	for i := 0; i < len(current); i += 2 {
		if i+2 > len(current) {
			next = append(next, current[i])
		} else {
			buf := bytes.Buffer{}
			buf.Write(current[i])
			buf.Write(current[i+1])
			bs, err := hex.DecodeString(Sha256(buf.Bytes()))
			if err != nil {
				LogCLI(err.Error(), 0)
			}
			next = append(next, bs)
		}
	}
	if len(next) > 1 {
		return Merkle(next)
	}
	if len(next) == 1 {
		return next
	}
	LogCLI("this should not happen", 0)
	return
}
