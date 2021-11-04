package global

import (
	"net"
	"time"
)

var ProxyTaskQueue map[string]map[string]ProxyTaskQueueItem

type ProxyTaskQueueItem struct {
	ID       string
	Conn     net.Conn
	TimeUnix int64 //任务队列超时判断
	Status   bool  //是否连接，未连接超时清除连接
}

func init() {
	ProxyTaskQueue = map[string]map[string]ProxyTaskQueueItem{}
}

//超时任务清理
func ProxyTaskQueueTimeoutClear(serverID string /*服务器ID*/, timeoutSeconds int64 /*超时秒数*/) {
	for {
		if _, ok := ProxyTaskQueue[serverID]; !ok {
			return
		}
		for id, item := range ProxyTaskQueue[serverID] {
			currTimeUnix := time.Now().Unix()
			//清理30秒未连接的超市连接
			if !item.Status && (currTimeUnix-item.TimeUnix) > timeoutSeconds {
				_ = item.Conn.Close()
			}
			delete(ProxyTaskQueue[serverID], id)
		}
		time.Sleep(time.Second * 5)
	}
}

//关闭服务ID任务队列
func ProxyTaskQueueClose(serverID string /*服务器ID*/) {
	if _, ok := ProxyTaskQueue[serverID]; !ok {
		return
	}
	for id, item := range ProxyTaskQueue[serverID] {
		_ = item.Conn.Close()
		delete(ProxyTaskQueue[serverID], id)
	}
	delete(ProxyTaskQueue, serverID)
}
