package utils

import (
	"net/url"
	"strings"
)

// url路径连接
// 使用url库ResolveReference方法URL后缀没有'/'路径会被省略
func UrlPathJoin(s string, path string) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + strings.TrimPrefix(path, "/")
	return u, err
}
