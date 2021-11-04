package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	uuid "github.com/satori/go.uuid"
	"strconv"
)

func Md5(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

func UUID() string {
	return fmt.Sprintf("%s", uuid.Must(uuid.NewV4()))
}

func ToInt64(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
