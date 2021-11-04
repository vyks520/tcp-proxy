package client

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	json "github.com/json-iterator/go"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"tcp-proxy/modules/jwt"
	. "tcp-proxy/modules/log"
	. "tcp-proxy/modules/types"
	"tcp-proxy/modules/utils"
	"time"
)

type Config struct {
	TaskAddr        string
	ServerID        string
	Secret          string
	ProxyTargetAddr string
}

type proxyClient struct {
	taskAddr        string
	serverID        string
	secret          string
	token           string
	proxyTargetAddr string
}

func NewClient(conf Config) *proxyClient {
	client := proxyClient{
		taskAddr:        conf.TaskAddr,
		serverID:        conf.ServerID,
		secret:          conf.Secret,
		proxyTargetAddr: conf.ProxyTargetAddr,
	}
	return &client
}

func (c *proxyClient) webSocket(u *url.URL) (*websocket.Conn, error) {
	reqHeader := http.Header{}
	reqHeader.Set("Authorization", c.token)
	sConn, _, err := websocket.DefaultDialer.Dial(u.String(), reqHeader)
	return sConn, err
}

func (c *proxyClient) ProxyRegister() error {
	info := jwt.EncryptSecret(jwt.LoginInfo{
		ServerID:     c.serverID,
		ClientSecret: c.secret,
		Timestamp:    time.Now().Unix(),
	})

	u, _ := utils.UrlPathJoin(c.taskAddr, fmt.Sprintf("/client-register/%s/%s/%d", info.ServerID, info.ClientSecret, info.Timestamp))
	sConn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		Logger.Errorf("代理服务器: %s, 连接创建失败, %s", u.String(), err.Error())
		return err
	}

	defer func() {
		_ = sConn.WriteControl(websocket.CloseMessage, []byte(""), time.Now())
		_ = sConn.Close()
	}()
	verifyData := TaskDataJson{}
	err = sConn.ReadJSON(&verifyData)
	if err != nil {
		Logger.Errorf("server_id: %s, 读取代理服务器验证数据时发生错误: %s", c.serverID, err.Error())
		return err
	}
	err = c.clientVerify(verifyData)
	if err != nil {
		return err
	}
	Logger.Infof("server_id: %s, 代理服务器: %s, 代理目标服务器: %s, 任务请求服务注册成功, 等待任务处理...", c.serverID, c.taskAddr, c.proxyTargetAddr)

	for {
		taskData := TaskDataJson{}
		err = sConn.ReadJSON(&taskData)
		if err != nil {
			Logger.Errorf("server_id: %s, 读取服务器任务列表错误: %s", c.serverID, err.Error())
			return err
		}

		switch taskData.Method {
		case "request":
			if taskData.Status == 0 {
				Logger.Debugf("server_id: %s, TaskID: %s, 接收到新请求", c.serverID, taskData.TaskID)
				go c.proxyHandler(taskData.TaskID)
			} else {
				Logger.Errorf("server_id: %s, taskID: %s, 代理服务端响应的任务状态异常, %s", c.serverID, taskData.TaskID, taskData.Msg)
			}
		case "verify":
			err = c.clientVerify(verifyData)
			if err != nil {
				return err
			}
		default:
			Logger.Errorf("server_id: %s, 收到的任务方法'%s'不支持", c.serverID, taskData.Method)
		}
	}
}

func (c *proxyClient) proxyHandler(taskID string) {
	proxyTargetConn, err := net.Dial("tcp", c.proxyTargetAddr)
	if err != nil {
		Logger.Errorf("代理目标服务器: %s, 连接创建失败, %s", c.proxyTargetAddr, err.Error())
		return
	}
	defer func() {
		_ = proxyTargetConn.Close()
		Logger.Debugf("server_id: %s, taskID: %s, 任务结束", c.serverID, taskID)
	}()

	u, _ := utils.UrlPathJoin(c.taskAddr, fmt.Sprintf("/task-handle/%s", taskID))
	sConn, err := c.webSocket(u)
	if err != nil {
		Logger.Errorf("代理服务器: %s, 连接创建失败, %s", u.String(), err.Error())
		return
	}
	defer func() {
		_ = sConn.Close()
	}()

	ch := make(chan bool)

	go func() {
		for {
			msgType, resData, err := sConn.ReadMessage()
			if err != nil {
				ch <- true
				//服务端正常关闭连接时客户端还在等待读取数据会出现1006错误，需排除此错误
				if err != io.EOF && !strings.Contains(err.Error(), "websocket: close 1006 (abnormal closure): unexpected EOF") {
					Logger.Errorf("server_id: %s, 代理服务器请求数据读取错误: %s", c.serverID, err.Error())
				}
				return
			}
			switch msgType {
			case 1:
				data := TaskDataJson{}
				err := json.Unmarshal(resData, &data)
				if err != nil {
					Logger.Errorf("server_id: %s, 解析服务器通知消息发生异常: %s", c.serverID, err.Error())
					ch <- true
					return
				}

				switch data.Method {
				case "notify":
					Logger.Error(data.Msg)
					ch <- true
					return
				case "verify":
					if data.Status != 0 {
						Logger.Error(data.Msg)
						ch <- true
						return
					}
				default:
					Logger.Errorf("server_id: %s, 方法不支持: %s", c.serverID, string(resData))
					ch <- true
					return
				}
			case 2:
				_, err = proxyTargetConn.Write(resData)
				if err != nil {
					ch <- true
					Logger.Errorf("server_id: %s, 向代理目标服务器写数据错误: %s", c.serverID, err.Error())
					return
				}
			default:
				ch <- true
				Logger.Errorf("server_id: %s, 服务端websocket消息类型不正确!", c.serverID)
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
					Logger.Errorf("server_id: %s, 读取代理目标服务器'%s'时发生数据错误: %s", c.serverID, c.proxyTargetAddr, err.Error())
				}
				return
			}
			err = sConn.WriteMessage(websocket.BinaryMessage, buffer[0:n])
			if err != nil {
				ch <- true
				Logger.Errorf("server_id: %s, 向代理服务器写数据错误: %s", c.serverID, err.Error())
				return
			}
		}
	}()
	<-ch
}

func (c *proxyClient) clientVerify(data TaskDataJson) error {
	if data.Method != "verify" {
		errMsg := fmt.Sprintf("server_id: %s, 代理服务器响应的验证方法不正确。", c.serverID)
		Logger.Error(errMsg)
		return errors.New(errMsg)
	}
	if data.Status != 0 {
		Logger.Error(data.Msg)
		return errors.New(data.Msg)
	}
	c.token = data.Token
	Logger.Infof("serverID: %s, token刷新成功", c.serverID)
	return nil
}
