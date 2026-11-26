package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordMemory = 19 * 1024
	passwordRounds = 2
	passwordLanes  = 1
	saltBytes      = 16
	hashBytes      = 32
)

var (
	passwordEncoding = base64.RawStdEncoding
	passwordPHC      = fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$", argon2.Version, passwordMemory, passwordRounds, passwordLanes)
)

func hashPassword(password string) string {
	salt := make([]byte, saltBytes)
	rand.Read(salt)
	hash := argon2.IDKey([]byte(password), salt, passwordRounds, passwordMemory, passwordLanes, hashBytes)
	return passwordPHC + passwordEncoding.EncodeToString(salt) + "$" + passwordEncoding.EncodeToString(hash)
}

func verifyPassword(password, stored string) bool {
	encoded, supported := strings.CutPrefix(stored, passwordPHC)
	saltPart, hashPart, found := strings.Cut(encoded, "$")
	salt, saltErr := passwordEncoding.DecodeString(saltPart)
	expected, hashErr := passwordEncoding.DecodeString(hashPart)
	if !supported || !found || saltErr != nil || hashErr != nil || len(salt) != saltBytes || len(expected) != hashBytes {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, passwordRounds, passwordMemory, passwordLanes, hashBytes)
	return subtle.ConstantTimeCompare(expected, actual) == 1
}
