package types

import (
	"crypto/sha512"
	"encoding/hex"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"k8s.io/apimachinery/pkg/api/resource"
)

func ConvertSize(size interface{}) (int64, error) {
	switch size := size.(type) {
	case int64:
		return size, nil
	case int:
		return int64(size), nil
	case string:
		if size == "" {
			return 0, nil
		}
		quantity, err := resource.ParseQuantity(size)
		if err != nil {
			return 0, errors.Wrapf(err, "error parsing size '%s'", size)
		}
		return quantity.Value(), nil
	}
	return 0, errors.Errorf("could not parse size '%v'", size)
}

func UUID() string {
	return uuid.NewV4().String()
}

func RandomID() string {
	return UUID()[:8]
}

func GetStringChecksum(data string) string {
	return GetChecksumSHA512([]byte(data))
}

func GetChecksumSHA512(data []byte) string {
	checksum := sha512.Sum512(data)
	return hex.EncodeToString(checksum[:])
}
