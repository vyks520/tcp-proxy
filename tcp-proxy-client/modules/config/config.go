package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

var Config serverConfig

type serverConfig struct {
	TaskAddr  string       `json:"task_addr"`
	LogLevel   string       `json:"log_level"`
	ProxyList []ProxyItem `json:"proxy_list"`
}

type ProxyItem struct {
	ServerID        string `json:"server_id"`
	ClientSecret    string `json:"client_secret"`
	ProxyTargetAddr string `json:"proxy_target_addr"`
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
}
