package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/kardianos/service"
	"net/http"
	"os"
	. "tcp-proxy/modules/log"
	. "tcp-proxy/tcp-proxy-server/modules/config"
	. "tcp-proxy/tcp-proxy-server/modules/controllers"
)

type program struct{}

func main() {
	LoggerInit(Config.LogLevel)
	svcConfig := &service.Config{
		Name:        "tcp-proxy-server", //服务显示名称
		DisplayName: "tcp-proxy-server", //服务名称
		Description: "tcp-proxy-server", //服务描述
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
			Logger.Println("服务安装成功")
			return
		}

		if os.Args[1] == "remove" {
			err := s.Uninstall()
			if err != nil {
				Logger.Fatalln(err.Error())
			}
			Logger.Info("服务卸载成功")
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
	gin.SetMode(gin.ReleaseMode)
	router := func() *gin.Engine {
		engine := gin.New()
		engine.Use(gin.LoggerWithFormatter(GinLogFormatter), gin.Recovery()) //自定义日志
		return engine
	}()
	router.Use(Cors())
	router.GET("/client-register/:serverID", ClientRegister)
	router.GET("/task-handle/:serverID/:taskID", TaskHandle)

	addr := fmt.Sprintf("%s:%d", Config.Host, Config.Port)
	Logger.Infof("服务运行在：%s", addr)
	err := http.ListenAndServe(addr, router)
	if err != nil {
		Logger.Error(fmt.Sprintf("服务启动失败：%s", err.Error()))
	}
}

