// +build linux

package eventloop

import (
	"io"

	"github.com/ikilobyte/netman/util"

	"github.com/ikilobyte/netman/iface"
	"golang.org/x/sys/unix"
)

type Poller struct {
	Epfd       int                   // eventpoll fd
	Events     []unix.EpollEvent     //
	ConnectMgr iface.IConnectManager //
}

//NewPoller 创建epoll
func NewPoller(connectMgr iface.IConnectManager) (*Poller, error) {

	fd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return nil, err
	}

	return &Poller{
		Epfd:       fd,
		Events:     make([]unix.EpollEvent, 128),
		ConnectMgr: connectMgr,
	}, nil
}

//Wait 等待消息到达，通过通道传递出去
func (p *Poller) Wait(emitCh chan<- iface.IRequest) {

	for {
		// n有三种情况，-1，0，> 0
		n, err := unix.EpollWait(p.Epfd, p.Events, -1)
		if err != nil {
			if err == unix.EAGAIN || err == unix.EINTR {
				continue
			}

			util.Logger.WithField("epfd", p.Epfd).WithField("error", err).Error("epoll_wait error")
			// 断开这个epoll管理的所有连接
			p.ConnectMgr.ClearByEpFd(p.Epfd)
			return
		}

		for i := 0; i < n; i++ {

			var (
				event  = p.Events[i]
				connFd = int(event.Fd)
				connID = int(event.Pad)
				conn   iface.IConnect
			)

			// 1、通过connID获取conn实例
			if conn = p.ConnectMgr.Get(connID); conn == nil {
				// 断开连接
				_ = unix.Close(connFd)
				_ = p.Remove(connFd)
				continue
			}

			// 2、读取一个完整的包
			message, err := conn.GetPacker().ReadFull(connFd)
			if err != nil {

				// 这两种情况可以直接断开连接
				if err == io.EOF || err == util.HeadBytesLengthFail {

					// 断开连接操作
					_ = conn.Close()
					_ = p.Remove(connFd)
					p.ConnectMgr.Remove(conn)
				}
				continue
			}

			// 3、将消息传递出去，交给worker处理
			if message.Len() <= 0 {
				continue
			}

			emitCh <- util.NewRequest(conn, message)
		}
	}
}

//AddRead 添加读事件
func (p *Poller) AddRead(fd, connID int) error {
	return unix.EpollCtl(p.Epfd, unix.EPOLL_CTL_ADD, fd, &unix.EpollEvent{
		Events: unix.EPOLLIN | unix.EPOLLPRI,
		Fd:     int32(fd),
		Pad:    int32(connID),
	})
}

//AddWrite 添加可写事件
func (p *Poller) AddWrite(fd, connID int) error {
	return unix.EpollCtl(p.Epfd, unix.EPOLL_CTL_ADD, fd, &unix.EpollEvent{
		Events: unix.EPOLLOUT,
		Fd:     int32(fd),
		Pad:    int32(connID),
	})
}

//Remove 删除某个fd的事件
func (p *Poller) Remove(fd int) error {
	return unix.EpollCtl(p.Epfd, unix.EPOLL_CTL_DEL, fd, nil)
}

//Close 关闭FD
func (p *Poller) Close() error {
	return unix.Close(p.Epfd)
}
