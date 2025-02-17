package iface

type IConnectManager interface {
	Get(connID int) IConnect
	Add(conn IConnect) int
	Remove(conn IConnect)
	Len() int
	ClearByEpFd(epfd int)
	ClearAll()
}
