package iface

type IPoller interface {
	AddRead(fd int, connID int) error
	AddWrite(fd, connID int) error
	Wait(emitCh chan<- IRequest)
	Remove(fd int) error
	Close() error
}
