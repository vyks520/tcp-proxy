package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
)

var Config serverConfig

type serverConfig struct {
	TaskAddr  string      `json:"task_addr"`
	LogLevel  string      `json:"log_level"`
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
	//兼容旧的配置方式
	if matched, _ := regexp.MatchString(`^\w*://`, Config.TaskAddr); !matched {
		Config.TaskAddr = fmt.Sprintf("ws://%s", Config.TaskAddr)
	}
	u, err := url.Parse(Config.TaskAddr)
	if err != nil {
		log.Panicf("config.json中task_addr连接地址配置不正确, %s", err.Error())
	}
	Config.TaskAddr = u.String()
}
