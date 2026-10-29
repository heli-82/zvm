package verify

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/jedisct1/go-minisign"
)

var InvalidSignature = errors.New("Invalid signature")
var InvalidComment = errors.New("Invalid comment")

func Minisig(filePath, minisigPath, publicKey string) error {
	pk, err := minisign.NewPublicKey(publicKey)
	if err != nil {
		return err
	}

	sigdata, err := os.ReadFile(minisigPath)
	if err != nil {
		return err
	}

	signature, err := minisign.DecodeSignature(string(sigdata))
	if err != nil {
		return err
	}
	valid, err := pk.VerifyFromFile(filePath, signature)
	if err != nil {
		return err
	}

	if !valid {
		return InvalidSignature
	}

	re := regexp.MustCompile(`^trusted comment: timestamp:\d+\tfile:(.*)\thashed$`)
	match := re.FindStringSubmatch(signature.TrustedComment)
	if len(match) != 2 {
		return errors.New("Trusted comment does not match")
	}
	if match[1] != filepath.Base(filePath) {
		return errors.New("Invalid filename")
	}

	return nil
}
