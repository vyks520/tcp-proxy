package controllers

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"io"
	"net"
	"net/http"
	"strings"
	. "tcp-proxy/modules/log"
	. "tcp-proxy/modules/types"
	. "tcp-proxy/modules/wsUpgrade"
	. "tcp-proxy/tcp-proxy-server/modules/config"
	. "tcp-proxy/tcp-proxy-server/modules/global"
	"time"
)

func ReqAuth(c *gin.Context, cConn *websocket.Conn) (ProxyItem, error) {
	var ok bool
	var serverInfo = ProxyItem{}
	serverID := c.Param("serverID")
	clientSecret := c.GetHeader("Authorization")
	serverInfo, ok = ServerMap[serverID]
	if !ok {
		errMsg := fmt.Sprintf("server_id'%s'不存在，禁止访问", serverID)
		Logger.Errorf(errMsg)
		err := cConn.WriteJSON(TaskDataJson{
			Status: 1,
			Msg:    fmt.Sprintf("URL: %s,  请求未授权，禁止访问！", c.Request.URL.String()),
			Method: "verify",
		})
		if err != nil {
			Logger.Errorf("server_id: %s, 响应代理客户端验证结果错误: %s", serverInfo.ServerID, err.Error())
		}
		return serverInfo, errors.New(errMsg)
	}
	if serverInfo.ClientSecret != clientSecret {
		errMsg := fmt.Sprintf("server_id'%s'密钥不正确，禁止访问", serverID)
		Logger.Errorf(errMsg)
		err := cConn.WriteJSON(TaskDataJson{
			Status: 1,
			Msg:    fmt.Sprintf("URL: %s, 请求未授权，禁止访问！", c.Request.URL.String()),
			Method: "verify",
		})
		if err != nil {
			Logger.Errorf("server_id: %s, 响应代理客户端验证结果错误: %s", serverInfo.ServerID, err.Error())
		}
		return serverInfo, errors.New(errMsg)
	}
	c.Set("serverInfo", serverInfo)
	return serverInfo, nil
}

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		//放行所有OPTIONS方法
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		// 处理请求
		c.Next()
	}
}

// 客户端注册，推送请求
func ClientRegister(c *gin.Context) {
	cConn, err := Upgrade(c.Writer, c.Request, 1024, 1024)
	if err != nil {
		Logger.Errorf("URL: %s,  连接错误: %s", c.Request.URL.String(), err.Error())
		return
	}
	defer func() {
		_ = cConn.Close()
	}()
	serverInfo, err := ReqAuth(c, cConn)
	if err != nil {
		return
	}
	if _, ok := ProxyTaskQueue[serverInfo.ServerID]; ok {
		err = cConn.WriteJSON(TaskDataJson{
			Status: 1,
			Msg:    fmt.Sprintf("server_id: %s, 已登录，不能重复登录，请退出其他已登录的代理客户端后重试！", serverInfo.ServerID),
			Method: "verify",
		})
		if err != nil {
			Logger.Errorf("server_id: %s, 响应代理客户端验证结果错误: %s", serverInfo.ServerID, err.Error())
			return
		}
	}

	sListener, err := net.Listen("tcp", serverInfo.ServerAddr)
	if err != nil {
		Logger.Errorf("server_id: %s,  监听'%s'时发生错误: %s", serverInfo.ServerID, serverInfo.ServerAddr, err.Error())
		err := cConn.WriteJSON(TaskDataJson{
			Status: 1,
			Msg:    fmt.Sprintf("server_id: %s,  监听'%s'时发生错误，请联系管理员！", serverInfo.ServerID, serverInfo.ServerAddr),
			Method: "verify",
		})
		if err != nil {
			Logger.Errorf("server_id: %s, 响应代理客户端验证结果错误: %s", serverInfo.ServerID, err.Error())
		}
		return
	}
	defer func() {
		_ = sListener.Close()
	}()
	Logger.Infof("server_id: %s, 代理服务器地址: %s, 服务已启动！", serverInfo.ServerID, serverInfo.ServerAddr)
	//初始化当前server_id任务列表
	ProxyTaskQueue[serverInfo.ServerID] = map[string]ProxyTaskQueueItem{}
	defer ProxyTaskQueueClose(serverInfo.ServerID)

	//清理10秒未处理的超时任务队列
	go ProxyTaskQueueTimeoutClear(serverInfo.ServerID, 10)

	err = cConn.WriteJSON(TaskDataJson{
		Status: 0,
		Msg:    "ok",
		Method: "verify",
	})
	if err != nil {
		Logger.Errorf("server_id: %s, 响应代理客户端验证结果错误: %s", serverInfo.ServerID, err.Error())
		return
	}

	go func() {
		for {
			reqConn, err := sListener.Accept()
			if err != nil {
				Logger.Errorf("server_id: %s,  读取代理服务器数据错误, 如果代理客户端主动断开连接，将会主动关闭连接，可以忽略此错误: %s", serverInfo.ServerID, err.Error())
				return
			}
			taskID := fmt.Sprintf("%s", uuid.Must(uuid.NewV4()))
			ProxyTaskQueue[serverInfo.ServerID][taskID] = ProxyTaskQueueItem{
				ID:       taskID,
				Conn:     reqConn,
				TimeUnix: time.Now().Unix(),
				Status:   false,
			}

			err = cConn.WriteJSON(TaskDataJson{
				Status: 1,
				Msg:    "课件会计科拮抗剂",
				Method: "request",
				TaskID: taskID,
			})
			if err != nil {
				Logger.Errorf("server_id: %s, WriteJSON error: %s", serverInfo.ServerID, err.Error())
				return
			}
			Logger.Debugf("server_id: %s, taskID: %s, 新请求推送成功！", serverInfo.ServerID, taskID)
		}
	}()
	for {
		_, _, err = cConn.ReadMessage()
		if err != nil {
			Logger.Infof("server_id: %s, 代理服务器地址: %s, 代理客户端端口连接, error: %s", serverInfo.ServerID, serverInfo.ServerAddr, err.Error())
			return
		}
	}
}

//接收客户端任务请求处理
func TaskHandle(c *gin.Context) {
	cConn, err := Upgrade(c.Writer, c.Request, 4096, 4096)
	if err != nil {
		Logger.Errorf("URL: %s,  连接错误: %s", c.Request.URL.String(), err.Error())
		return
	}
	defer func() {
		_ = cConn.Close()
	}()

	serverInfo, err := ReqAuth(c, cConn)
	if err != nil {
		return
	}

	taskID := c.Param("taskID")
	task, ok := ProxyTaskQueue[serverInfo.ServerID][taskID]
	if !ok {
		errMsg := fmt.Sprintf("server_id: %s, 请求的任务'%s'不存在！", serverInfo.ServerID, taskID)
		Logger.Error(errMsg)
		err := cConn.WriteJSON(TaskDataJson{
			Status: 1,
			Msg:    errMsg,
			Method: "notify",
		})
		if err != nil {
			Logger.Error(errMsg)
		}
		return
	}
	defer func() {
		_ = task.Conn.Close()
		delete(ProxyTaskQueue[serverInfo.ServerID], taskID)
		Logger.Debugf("server_id: %s, taskID: %s, 连接已关闭！", serverInfo.ServerID, taskID)
	}()

	ch := make(chan bool)

	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := task.Conn.Read(buffer)
			if err != nil {
				ch <- true
				if err != io.EOF {
					Logger.Errorf("server_id: %s, 读取'%s'数据错误: %s", serverInfo.ServerID, serverInfo.ServerAddr, err.Error())
				}
				return
			}
			err = cConn.WriteMessage(websocket.BinaryMessage, buffer[:n])
			if err != nil {
				ch <- true
				Logger.Errorf("server_id: %s, 向代理客户端写数据出错: %s", serverInfo.ServerID, err.Error())
				return
			}
		}
	}()

	go func() {
		for {
			msgType, data, err := cConn.ReadMessage()
			if err != nil {
				ch <- true
				if err != io.EOF && !strings.Contains(err.Error(), "websocket: close 1006 (abnormal closure): unexpected EOF") {
					Logger.Errorf("server_id: %s, 从代理客户端数据读取错误: %s", serverInfo.ServerID, err.Error())
				}
				return
			}
			if msgType != 2 {
				ch <- true
				Logger.Errorf("server_id: %s, 代理客户端websocket消息类型不正确: %s", serverInfo.ServerID, err.Error())
				continue
			}
			_, err = task.Conn.Write(data)
			if err != nil {
				ch <- true
				Logger.Errorf("server_id: %s, 写数据出错: %s", serverInfo.ServerID, err.Error())
				return
			}
		}
	}()
	<-ch
}
