// Connector
package smpp

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	_ int32 = iota
	CONN_CONNECTED
	CONN_ACTIVE
	CONNECT_CLOSED
	CONN_ERROR
)

const (
	EventType = iota
	AcceptEvent
	CloseEvent
	ErrorEvent
	ConnectEvent
)

const (
	KNOWN_MSG_TYPE = 0x255
)

var ERROR_NOFINDHANDLE = errors.New("No find error")
var ERROR_NOAUTHMSG = errors.New("Recieve Msg Before Auth OK")

type MsgHandle interface {
	Handle(c *Connector, msg *SmppMsg) error
}

type UnknownMsgHandle interface {
	UnknownMsgHandle(c *Connector, msg *SmppMsg) error
}

type UnknownRespMsgHandle interface {
	UnknownRespMsgHandle(c *Connector, msg *SmppMsg) error
}

type MsgFunc func(c *Connector, msg *SmppMsg) error

func (f MsgFunc) Handle(c *Connector, msg *SmppMsg) error {
	return f(c, msg)
}

type Eventhandle func(conn *Connector, etype int) (retcode int)

type Connector struct {
	conn net.Conn
	seq  uint32
	//rcvevent   chan SmppMsg
	sendevent  chan SmppMsg
	done       chan struct{}
	sessions   sessionmap
	lasterror  error
	status     int32
	rcvroutine IOHandle
	sndroutine IOHandle
	onevent    Eventhandle
	log        func(args ...interface{})
	owner      interface{}
}

type HandlerMux interface {
	RegHandleFunc(cmd uint32, f MsgFunc)
	RegHandle(cmd uint32, handle MsgHandle)
}

type IOHandle func(cn *Connector)

type sessionmap struct {
	lock sync.RWMutex
	smap map[uint32]session
}

func (mp *sessionmap) Init() {
	mp.smap = make(map[uint32]session)
}

func (mp *sessionmap) NewSession(seq uint32) *session {
	mp.lock.Lock()
	defer mp.lock.Unlock()
	var s session
	s.starttm = time.Now()
	s.ch = make(chan SmppMsg)
	mp.smap[seq] = s
	return &s
}

func (mp *sessionmap) ClearSession(seq uint32) error {
	mp.lock.Lock()
	defer mp.lock.Unlock()
	s, ok := mp.smap[seq]
	if ok {
		delete(mp.smap, seq)
		close(s.ch)
		return nil
	}
	return fmt.Errorf("session no found %d", seq)
}

func (mp *sessionmap) ClearTimeoutSession(t time.Duration) {
	mp.lock.Lock()
	defer mp.lock.Unlock()
	now := time.Now()
	for k, v := range mp.smap {
		if now.Sub(v.starttm) > t {
			delete(mp.smap, k)
		}
	}
}

type session struct {
	ch      chan SmppMsg
	starttm time.Time
}

func NewConnector(conn net.Conn) *Connector {
	rt := &Connector{conn: conn}
	rt.sessions.Init()
	rt.seq = 0
	rt.status = CONN_CONNECTED
	//rt.rcvevent = make(chan SmppMsg, 1000)
	rt.sendevent = make(chan SmppMsg, 1000)
	rt.done = make(chan struct{})
	return rt
}

func (cn *Connector) RemoteAddr() net.Addr {
	return cn.conn.RemoteAddr()
}

func (cn *Connector) GetOwner() interface{} {
	return cn.owner
}

func (cn *Connector) SetOwner(owner interface{}) {

	cn.owner = owner
}

func (cn *Connector) Start() {
	if cn.rcvroutine == nil {
		go cn.rcvRoutine()
	} else {
		go cn.rcvroutine(cn)
	}

	if cn.sndroutine == nil {
		go cn.sendRoutine()
	} else {
		go cn.sndroutine(cn)
	}

}

func (cn *Connector) SetSendRoutine(handle IOHandle) {
	cn.sndroutine = handle
}

func (cn *Connector) SetRcvRoutine(handle IOHandle) {
	cn.rcvroutine = handle
}

func (cn *Connector) SetLogHandle(handle func(args ...interface{})) {
	cn.log = handle
}

func (cn *Connector) Log(args ...interface{}) {
	if cn.log != nil {
		cn.log(args...)
	}
}

func (cn *Connector) logf(fm string, args ...interface{}) {
	cn.Log(fmt.Sprintf(fm, args...))
}

func (cn *Connector) Close() {
	status := atomic.SwapInt32(&cn.status, CONNECT_CLOSED)
	if status != CONNECT_CLOSED {
		cn.logf("connect.close  raddr[%v] addr[%v]", cn.conn.RemoteAddr(), cn.conn.LocalAddr())
		close(cn.done)
		cn.emitClose()
		cn.conn.Close()
	}
}

func (cn *Connector) CloseError(err error) {
	st := atomic.LoadInt32(&cn.status)
	if st != CONNECT_CLOSED {
		cn.lasterror = err
		cn.Close()
	}
}

func (cn *Connector) emitEror(err error) {
	ok := atomic.CompareAndSwapInt32(&cn.status, CONN_ACTIVE, CONN_ERROR)

	if ok {
		cn.lasterror = err
		if cn.onevent != nil {
			cn.onevent(cn, ErrorEvent)
		}
		cn.Close()
		return
	}

	ok = atomic.CompareAndSwapInt32(&cn.status, CONN_CONNECTED, CONN_ERROR)

	if ok {
		cn.lasterror = err
		if cn.onevent != nil {
			cn.onevent(cn, ErrorEvent)
		}
		cn.Close()
		return
	}
}

func (cn *Connector) SetEventHandle(e Eventhandle) {
	cn.onevent = e
}

func (cn *Connector) emitConnect() {
	status := atomic.SwapInt32(&cn.status, CONN_CONNECTED)
	if status != CONN_CONNECTED {
		if cn.onevent != nil {
			cn.onevent(cn, ConnectEvent)
		}
	}
}

func (cn *Connector) emitClose() {
	ok := atomic.CompareAndSwapInt32(&cn.status, CONN_ACTIVE, CONNECT_CLOSED)
	if ok {
		if cn.onevent != nil {
			cn.onevent(cn, CloseEvent)
		}
	}

	ok = atomic.CompareAndSwapInt32(&cn.status, CONN_CONNECTED, CONNECT_CLOSED)
	if ok {
		if cn.onevent != nil {
			cn.onevent(cn, CloseEvent)
		}
	}

}

func (cn *Connector) sendRoutine() {
	cn.logf("start SendRoute cn[%p]", cn)
	defer cn.logf("exit sendRoutine cn[%p]", cn)

	ch := cn.sendevent
	writer := bufio.NewWriter(cn.conn)
	for {
		select {
		case <-cn.done:
			return
		default:

		}
		select {

		case msg, ok := <-ch:
			if !ok {
				//cn.Close()
				//cn.lasterror = errors.New("send chan has closed")
				cn.emitEror(errors.New("send chan has closed"))
				return
			}
			cn.Log("write", msg)
			_, err := writer.Write(msg.Header.Data())
			if err != nil {
				cn.emitEror(fmt.Errorf("write header err %s", err))
				return
			}
			_, err = writer.Write(msg.Body.Data())
			if err != nil {
				cn.emitEror(fmt.Errorf("write body err %s", err))
				return
			}
		case <-time.After(time.Millisecond):
			writer.Flush()

		}

	}

}

func (cn *Connector) rcvRoutine() {
	defer cn.logf("exit rcvRoutine cn[%p]", cn)
	cn.logf("start rcvRoutine cn[%p]", cn)

	reader := bufio.NewReader(cn.conn)
	for {

		select {
		case <-cn.done:
			return
		default:
		}

		var header SmppHeader
		_, err := header.ReadFromIO(reader)
		if err != nil {
			cn.emitEror(fmt.Errorf("read header err %s", err))
			return
		}
		var body SmppBody
		_, err = body.ReadFromIO(reader, int(header.GetPduLen()))
		if err != nil {
			cn.emitEror(fmt.Errorf("read body err %s", err))
			return
		}

		cn.Log("rcv", header, body)
		cmd := header.GetCommandID()
		if IsReqCommand(cmd) == false {
			issession := func() bool {
				cn.sessions.lock.Lock()
				defer cn.sessions.lock.Unlock()
				rspchan, ok := cn.sessions.smap[header.GetSeq()]
				if ok {

					go func() {
						rspchan.ch <- SmppMsg{header, body}
						close(rspchan.ch)
					}()

					delete(cn.sessions.smap, header.GetSeq())
					return true
				}
				return false
			}()
			if issession {
				continue
			} else {
				if h, ok := cn.owner.(UnknownRespMsgHandle); ok {
					h.UnknownRespMsgHandle(cn, &SmppMsg{header, body})
				}

				continue
			}
		}

		if f, ok := cn.owner.(MsgHandle); ok {

			f.Handle(cn, &SmppMsg{header, body})
			continue

		}

	}

}

func (cn *Connector) sendRequst(msg SmppMsg, timeout time.Duration) (rsp *SmppMsg, err error) {
	if st := atomic.LoadInt32(&cn.status); st == CONNECT_CLOSED || st == CONN_ERROR {
		return nil, fmt.Errorf("connect not active")
	}

	newseq := atomic.AddUint32(&cn.seq, 1)
	msg.Header.SetSeq(newseq)
	msg.Header.SetPduLen(uint32(len(msg.Body.data)))

	s := cn.sessions.NewSession(newseq)

	cn.sendevent <- msg
	select {
	case rcv := <-s.ch:
		return &rcv, nil
	case <-time.After(timeout):
		cn.sessions.ClearSession(newseq)
		return nil, errors.New("wait reply session timeout")
	}

	return nil, nil
}

func (cn *Connector) BindReq(bdtype uint32, bd *Smpp_bind) (rsp *SmppMsg, err error) {
	var msg SmppMsg
	msg.Header.SetCommandID(bdtype)
	msg.Body = *NewBind(bd)
	return cn.sendRequst(msg, time.Second*5)
}

func (cn *Connector) RequstSm(smtype uint32, sm *Smpp_submit_sm, args ...TLV) (rsp *SmppMsg, err error) {
	var msg SmppMsg
	msg.Header.SetCommandID(smtype)
	msg.Body = *NewSubmitSM(sm, args...)
	return cn.sendRequst(msg, time.Second*5)
}

func (cn *Connector) ReplyfromMsg(srchd SmppHeader, rtcode uint32, rep SmppBody) (err error) {

	if st := atomic.LoadInt32(&cn.status); st == CONNECT_CLOSED || st == CONN_ERROR {
		return fmt.Errorf("Connector has closed")
	}

	var rply SmppMsg

	rply.Body = rep
	rply.Header.SetCommandID(srchd.GetCommandID() | 0x80000000)
	rply.Header.SetSeq(srchd.GetSeq())
	rply.Header.SetPduLen(uint32(len(rep.Data())))
	rply.Header.SetStatus(rtcode)
	select {
	case cn.sendevent <- rply:
		return nil
	case <-time.After(time.Millisecond * 100):
		return fmt.Errorf("reply msg send time out")
	}

}
