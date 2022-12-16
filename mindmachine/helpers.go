package mindmachine

import (
	"math/big"
	"os"
)

func Touch(path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
	}
	return nil
}

// check for path traversal and correct forward slashes

func Permille(signed, total int64) int64 {
	if signed > total {
		LogCLI("This should not happen", 0)
	}
	s := new(big.Rat)
	s.SetFrac64(signed, total)
	m := new(big.Rat)
	m.SetInt64(1000)
	s = s.Mul(s, m)
	i := s.Num()
	return int64(i.Int64())
}

//Contains checks if a slice contains a string
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
