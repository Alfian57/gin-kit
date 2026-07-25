// Package password provides Argon2id password hashing with encoded parameters.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

type Parameters struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	KeyLength   uint32
	SaltLength  uint32
}

var DefaultParameters = Parameters{Memory: 19 * 1024, Iterations: 2, Parallelism: 1, KeyLength: 32, SaltLength: 16}

func Hash(plain string) (string, error) { return New(DefaultParameters).Hash(plain) }

type Hasher struct{ parameters Parameters }

func New(parameters Parameters) Hasher {
	if parameters.Memory == 0 {
		parameters = DefaultParameters
	}
	if parameters.SaltLength == 0 {
		parameters.SaltLength = DefaultParameters.SaltLength
	}
	if parameters.KeyLength == 0 {
		parameters.KeyLength = DefaultParameters.KeyLength
	}
	return Hasher{parameters: parameters}
}

func (h Hasher) Hash(plain string) (string, error) {
	salt := make([]byte, h.parameters.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(plain), salt, h.parameters.Iterations, h.parameters.Memory, h.parameters.Parallelism, h.parameters.KeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		h.parameters.Memory, h.parameters.Iterations, h.parameters.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(sum)), nil
}

func Compare(plain, encoded string) bool {
	parameters, salt, expected, ok := parse(encoded)
	if !ok {
		return false
	}
	actual := argon2.IDKey([]byte(plain), salt, parameters.Iterations, parameters.Memory, parameters.Parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (h Hasher) NeedsRehash(encoded string) bool {
	parameters, _, _, ok := parse(encoded)
	if !ok {
		return true
	}
	return parameters.Memory != h.parameters.Memory || parameters.Iterations != h.parameters.Iterations ||
		parameters.Parallelism != h.parameters.Parallelism || parameters.KeyLength != h.parameters.KeyLength
}

func parse(encoded string) (Parameters, []byte, []byte, bool) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return Parameters{}, nil, nil, false
	}
	values := map[string]string{}
	for _, item := range strings.Split(parts[3], ",") {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			return Parameters{}, nil, nil, false
		}
		values[key] = value
	}
	memory, err1 := strconv.ParseUint(values["m"], 10, 32)
	iterations, err2 := strconv.ParseUint(values["t"], 10, 32)
	parallelism, err3 := strconv.ParseUint(values["p"], 10, 8)
	salt, err4 := base64.RawStdEncoding.DecodeString(parts[4])
	expected, err5 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil ||
		memory == 0 || iterations == 0 || parallelism == 0 || len(salt) < 8 || len(expected) < 16 {
		return Parameters{}, nil, nil, false
	}
	return Parameters{Memory: uint32(memory), Iterations: uint32(iterations), Parallelism: uint8(parallelism), KeyLength: uint32(len(expected)), SaltLength: uint32(len(salt))}, salt, expected, true
}

var ErrInvalidHash = errors.New("invalid password hash")
