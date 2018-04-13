// Server
package smpp

import (
	//"bufio"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	Addr     string
	OnAccept Eventhandle
	OnClosed Eventhandle
	//OnBind   Messagehandle
	log func(args ...interface{})

	funcmap map[uint32]MsgHandle
	maplock sync.RWMutex
}

func (svr *Server) RegHandleFunc(cmd uint32, f MsgFunc) {
	svr.RegHandle(cmd, MsgHandle(f))

}

func (svr *Server) RegHandle(cmd uint32, handle MsgHandle) {
	svr.maplock.Lock()
	defer svr.maplock.Unlock()
	if svr.funcmap == nil {
		svr.funcmap = make(map[uint32]MsgHandle)
	}

	switch CMDId(cmd) {

	case BIND_RECEIVER, BIND_TRANSCEIVER, BIND_TRANSMITTER:

		wrap := func(cn *Connector, msg *SmppMsg) error {

			err := handle.Handle(cn, msg)

			if err == nil {
				atomic.StoreInt32(&cn.status, CONN_ACTIVE)
			}
			return err
		}

		svr.funcmap[cmd] = MsgFunc(wrap)
		return
	}

	wrap := func(cn *Connector, msg *SmppMsg) error {
		var err error
		if cn.status == CONN_ACTIVE {
			err = handle.Handle(cn, msg)
		} else {
			err = svr.UnknownMsgHandle(cn, msg)
		}
		return err
	}

	svr.funcmap[cmd] = MsgFunc(wrap)

}

const (
	Accept_RT int = iota
	Refuse_RT
)

//type Messagehandle func(conn *Connector, header *SmppHeader, body *SmppBody) (retcode int, err error)

func (svr *Server) SetLogHandle(f func(args ...interface{})) {
	svr.log = f
}

func (svr *Server) Run() {
	listen, err := net.Listen("tcp", svr.Addr)

	if err != nil {
		fmt.Println(err)
		return
	}
	svr.Log("start listen", svr.Addr)
	for {
		listen.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second * 5))
		cn, err := listen.Accept()
		if err != nil {
			continue
		}

		conn := NewConnector(cn)
		conn.SetLogHandle(svr.log)
		conn.SetOwner(svr)
		if svr.OnAccept != nil {
			retcode := svr.OnAccept(conn, AcceptEvent)
			if retcode != Accept_RT {
				svr.Log("refuse  connect raddr[%v] ", conn.conn.RemoteAddr())
				conn.Close()
				continue
			}
		}
		conn.onevent = func(c *Connector, t int) int {
			switch t {
			case CloseEvent, ErrorEvent:
				if svr.OnClosed != nil {
					return svr.OnClosed(c, t)
				}
			}
			return 0
		}
		//conn.rcvroutine = svr.getrcvroutine()
		conn.Start()

	}
}

func (svr *Server) Handle(cn *Connector, msg *SmppMsg) error {
	cmd := msg.Header.GetCommandID()

	svr.maplock.RLock()
	defer svr.maplock.RUnlock()
	f, ok := svr.funcmap[cmd]
	if !ok {

		Queue(func() { svr.UnknownMsgHandle(cn, msg) })
		return ERROR_NOFINDHANDLE
	}

	Queue(func() { f.Handle(cn, msg) })

	return nil

}

func (svr *Server) UnknownMsgHandle(cn *Connector, msg *SmppMsg) error {
	cn.ReplyfromMsg(msg.Header, KNOWN_MSG_TYPE, SmppBody{})
	return ERROR_NOFINDHANDLE
}

func (svr *Server) UnknownRespMsgHandle(cn *Connector, msg *SmppMsg) error {
	return nil
}

//func (svr *Server) getrcvroutine() IOHandle {

//	return func(cn *Connector) {
//		defer close(cn.rcvevent)
//		reader := bufio.NewReader(cn.conn)
//		for {

//			select {
//			case <-cn.done:
//				//rcv exit event
//				return
//			default:
//			}

//			var header SmppHeader
//			_, err := header.ReadFromIO(reader)
//			if err != nil {
//				cn.CloseError(fmt.Errorf("read header err %s", err))
//				cn.Log(err)
//				return
//			}
//			var body SmppBody
//			_, err = body.ReadFromIO(reader, int(header.GetPduLen()))
//			if err != nil {
//				cn.CloseError(fmt.Errorf("read body err %s", err))
//				cn.Log(err)
//				return
//			}

//			cmd := header.GetCommandID()

//			switch CMDId(cmd) {
//			case BIND_RECEIVER, BIND_TRANSMITTER, BIND_TRANSCEIVER:
//				if svr.OnBind != nil {
//					rt, err := svr.OnBind(cn, &header, &body)
//					if rt == 0 {
//						cn.status = CONN_ACTIVE
//						continue
//					}
//					//认证失败
//					cn.lasterror = err
//					cn.Close()
//				}

//			default:
//				if cn.status != CONN_ACTIVE {
//					//skip all msg if status is no active
//					continue
//				}

//			}

//			if IsReqCommand(cmd) == false {
//				issession := func() bool {
//					cn.sessions.lock.Lock()
//					defer cn.sessions.lock.Unlock()
//					rspchan, ok := cn.sessions.smap[header.GetSeq()]
//					if ok {
//						rspchan.ch <- SmppMsg{header, body}
//						close(rspchan.ch)
//						delete(cn.sessions.smap, header.GetSeq())
//						return true
//					}
//					return false
//				}()
//				if issession {
//					continue
//				}
//			}

//			//cli.in <- SmppMsg{header, body}
//			cn.rcvevent <- SmppMsg{header, body}
//		}

//	}

//}

func (svr *Server) Log(args ...interface{}) {
	if svr.log != nil {
		svr.log(args...)
	}
}

func (svr *Server) Logf(f string, args ...interface{}) {

	svr.Log(fmt.Sprintf(f, args...))
}
