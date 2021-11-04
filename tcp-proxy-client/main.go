package main

import (
	"github.com/kardianos/service"
	"os"
	. "tcp-proxy/modules/log"
	"tcp-proxy/tcp-proxy-client/modules/client"
	. "tcp-proxy/tcp-proxy-client/modules/config"
	"time"
)

type program struct{}

func main() {
	LoggerInit(Config.LogLevel, Config.LogOutput)
	svcConfig := &service.Config{
		Name:        "tcp-proxy-client", //服务显示名称
		DisplayName: "tcp-proxy-client", //服务名称
		Description: "tcp-proxy-client", //服务描述
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		Logger.Fatalln(err)
	}

	if len(os.Args) > 1 {
		if os.Args[1] == "install" {
			err := s.Install()
			if err != nil {
				Logger.Fatal(err.Error())
			}
			Logger.Println("服务安装成功！")
			return
		}

		if os.Args[1] == "remove" {
			err := s.Uninstall()
			if err != nil {
				Logger.Fatalln(err.Error())
			}
			Logger.Info("服务卸载成功！")
			return
		}
	}

	err = s.Run()
	if err != nil {
		Logger.Fatal(err.Error())
	}
}

func (p *program) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *program) Stop(s service.Service) error {
	return nil
}

func (p *program) run() {
	ProxyStart()
}

func ProxyStart() {
	if len(Config.ProxyList) == 0 {
		Logger.Error("代理信息配置列表不能为空！")
		os.Exit(1)
	}
	for _, proxyInfo := range Config.ProxyList {
		go func(conf ProxyItem) {
			var err error
			proxyClient := client.NewClient(client.Config{
				TaskAddr:        Config.TaskAddr,
				ServerID:        conf.ServerID,
				Secret:          conf.ClientSecret,
				ProxyTargetAddr: conf.ProxyTargetAddr,
			})
			for {
				err = proxyClient.ProxyRegister()
				if err != nil {
					Logger.Errorf("代理服务器任务注册错误，10秒钟后重新连接...")
				}
				time.Sleep(time.Second * 10)
			}
		}(proxyInfo)
	}
}
