package main

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	json "github.com/json-iterator/go"
	"github.com/kardianos/service"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	. "tcp-proxy/modules/log"
	. "tcp-proxy/modules/types"
	"tcp-proxy/modules/utils"
	."tcp-proxy/tcp-proxy-client/modules/config"
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
	ProxyStart()
}

func ProxyStart() {
	if len(Config.ProxyList) == 0 {
		Logger.Error("代理信息配置列表不能为空！")
		os.Exit(1)
	}
	for _, proxyInfo := range Config.ProxyList {
		go func(proxyInfo ProxyItem) {
			var err error
			for {
				err = ProxyRegister(proxyInfo)
				if err != nil {
					Logger.Errorf("代理注册方法错误，10秒钟后重新连接！")
				}
				time.Sleep(time.Second * 10)
			}
		}(proxyInfo)
	}
}

func ProxyRegister(proxyInfo ProxyItem) error {
	//添加请求头授权信息
	reqHeader := http.Header{}
	reqHeader.Set("Authorization", proxyInfo.ClientSecret)

	u, _ := utils.UrlPathJoin(Config.TaskAddr, fmt.Sprintf("/client-register/%s", proxyInfo.ServerID))
	sConn, _, err := websocket.DefaultDialer.Dial(u.String(), reqHeader)
	if err != nil {
		Logger.Errorf("代理服务器: %s, 连接创建失败, %s", u.String(), err.Error())
		return err
	}

	defer func() {
		_ = sConn.Close()
	}()
	verifyData := TaskDataJson{}
	err = sConn.ReadJSON(&verifyData)
	if err != nil {
		Logger.Errorf("server_id: %s, 读取代理服务器验证数据时发生错误: %s", proxyInfo.ServerID, err.Error())
		return err
	}
	if verifyData.Method != "verify" {
		errMsg := fmt.Sprintf("server_id: %s, 代理服务器响应的验证方法不正确。")
		Logger.Error(errMsg)
		return errors.New(errMsg)
	}
	if verifyData.Status != 0 {
		Logger.Error(verifyData.Msg)
		return errors.New(verifyData.Msg)
	}
	Logger.Infof("server_id: %s, 代理服务器: %s, 代理目标服务器: %s, 任务请求服务注册成功, 等待任务处理...", proxyInfo.ServerID, Config.TaskAddr, proxyInfo.ProxyTargetAddr)
	for {
		taskData := TaskDataJson{}
		err = sConn.ReadJSON(&taskData)
		if err != nil {
			Logger.Errorf("server_id: %s, 读取服务器任务列表错误: %s", proxyInfo.ServerID, err.Error())
			return err
		}

		switch taskData.Method {
		case "request":
			if taskData.Status != 0 {
				Logger.Debugf("server_id: %s, TaskID: %s, 接收到新请求！", proxyInfo.ServerID, taskData.TaskID)
				go ProxyHandler(taskData.TaskID, proxyInfo)
			} else {
				Logger.Errorf("server_id: %s, taskID: %s, 代理服务端响应的任务状态异常, %s", proxyInfo.ServerID, taskData.TaskID, taskData.Msg)
			}
		default:
			Logger.Errorf("server_id: %s, 收到的任务方法'%s'不支持！", proxyInfo.ServerID, taskData.Method)
		}
	}
}

func ProxyHandler(taskID string, proxyInfo ProxyItem) {
	//添加请求头授权信息
	reqHeader := http.Header{}
	reqHeader.Set("Authorization", proxyInfo.ClientSecret)

	//URL添加serverID及任务ID
	u, _ := utils.UrlPathJoin(Config.TaskAddr, fmt.Sprintf("/task-handle/%s/%s", proxyInfo.ServerID, taskID))
	sConn, _, err := websocket.DefaultDialer.Dial(u.String(), reqHeader)
	if err != nil {
		Logger.Errorf("代理服务器: %s, 连接创建失败, %s", u.String(), err.Error())
		return
	}
	defer func() {
		_ = sConn.Close()
	}()

	targetAddr := proxyInfo.ProxyTargetAddr
	proxyTargetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		Logger.Errorf("代理目标服务器: %s, 连接创建失败, %s", targetAddr, err.Error())
		return
	}
	defer func() {
		_ = proxyTargetConn.Close()
		Logger.Debugf("server_id: %s, taskID: %s, 任务结束！", proxyInfo.ServerID, taskID)
	}()

	ch := make(chan bool)

	go func() {
		for {
			msgType, data, err := sConn.ReadMessage()
			if err != nil {
				ch <- true
				//服务端正常关闭连接时客户端还在等待读取数据会出现1006错误，需排除此错误
				if err != io.EOF && !strings.Contains(err.Error(), "websocket: close 1006 (abnormal closure): unexpected EOF") {
					Logger.Errorf("server_id: %s, 代理服务器请求数据读取错误: %s", proxyInfo.ServerID, err.Error())
				}
				return
			}
			switch msgType {
			case 1:
				notifyData := TaskDataJson{}
				err := json.Unmarshal(data, &notifyData)
				if err != nil {
					Logger.Errorf("server_id: %s, 解析服务器通知消息发生异常: %s", proxyInfo.ServerID, err.Error())
					ch <- true
					return
				}
				if notifyData.Method != "notify" {
					Logger.Errorf("server_id: %s, 方法不支持: %s", proxyInfo.ServerID, string(data))
					ch <- true
					return
				}
				Logger.Error(notifyData.Msg)
				ch <- true
				return
			case 2:
				_, err = proxyTargetConn.Write(data)
				if err != nil {
					ch <- true
					Logger.Errorf("server_id: %s, 向代理目标服务器写数据错误: %s", proxyInfo.ServerID, err.Error())
					return
				}
			default:
				ch <- true
				Logger.Errorf("server_id: %s, 服务端websocket消息类型不正确!", proxyInfo.ServerID)
				return
			}
		}
	}()

	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := proxyTargetConn.Read(buffer)
			if err != nil {
				ch <- true
				if err != io.EOF {
					Logger.Errorf("server_id: %s, 读取代理目标服务器'%s'时发生数据错误: %s", proxyInfo.ServerID, proxyInfo.ProxyTargetAddr, err.Error())
				}
				return
			}
			err = sConn.WriteMessage(websocket.BinaryMessage, buffer[0:n])
			if err != nil {
				ch <- true
				Logger.Errorf("server_id: %s, 向代理服务器写数据错误: %s", proxyInfo.ServerID, err.Error())
				return
			}
		}
	}()
	<-ch
}
