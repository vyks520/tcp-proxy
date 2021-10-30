package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

var Config serverConfig
var ServerMap serverMap

type serverMap map[string]ProxyItem

type serverConfig struct {
	Host       string       `json:"host"`
	Port       int          `json:"port"`
	LogLevel   string       `json:"log_level"`
	ProxyList []ProxyItem `json:"proxy_list"`
}

type ProxyItem struct {
	ServerID     string `json:"server_id"`
	ServerAddr   string `json:"server_addr"`
	ClientSecret string `json:"client_secret"`
}

func init() {
	appPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Panicf("服务执行路径获取失败：%s", err.Error())
	}
	bs, err := ioutil.ReadFile(appPath + "/config.json")
	if err != nil {
		log.Panicf("配置读取文件失败：%s", err.Error())
	}
	err = json.Unmarshal(bs, &Config)
	if err != nil {
		log.Panicf("config.json配置文件解析失败：%s", err.Error())
	}
	ServerMap = serverMap{}
	for _, item := range Config.ProxyList {
		ServerMap[item.ServerID] = item
	}
}
