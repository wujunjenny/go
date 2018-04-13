// muxandsession.go
package smgp

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type HandlerMux interface {
	RegHandleFunc(cmd CMDId, f MsgFunc)
	RegHandle(cmd CMDId, handle MsgHandle) //call by app
	RegDefaultHandle(handle MsgHandle)
	MsgHandle //call by underline
}

type BasicMux struct {
	handles        map[uint32]MsgHandle
	default_handle MsgHandle
	lock           sync.RWMutex
}

func (mux *BasicMux) RegHandle(cmd CMDId, handle MsgHandle) {
	mux.lock.Lock()
	mux.handles[cmd.ID()] = handle
	mux.lock.Unlock()
}

func (mux *BasicMux) RegHandleFunc(cmd CMDId, f MsgFunc) {
	mux.lock.Lock()
	mux.handles[cmd.ID()] = MsgHandle(f)
	mux.lock.Unlock()
}

func (mux *BasicMux) RegDefaultHandle(h MsgHandle) {
	mux.default_handle = h
}

//called by underline to active msg handle
func (mux *BasicMux) Handle(c *Connector, msg *PduMsg) error {
	id := msg.Header.GetCommandID().ID()

	f, ok := mux.handles[id]

	if ok {
		Queue(func() {
			f.Handle(c, msg)
		})
		return nil
	} else {
		if mux.default_handle != nil {
			Queue(func() {
				mux.default_handle.Handle(c, msg)
			})
			return nil
		}
	}

	return fmt.Errorf("UnKnown msg hanfle by id:%v", id)
}

type BasicSessionMap struct {
	sessions map[uint32]Session
	seq      uint32
	lock     sync.Mutex
}

func NewSessionMap() SessionMap {

	return &BasicSessionMap{
		seq: 0,
	}

}

var sessionpool = sync.Pool{New: func() interface{} {
	return &BasicSession{
		seq:   0,
		event: make(chan struct{}, 1),
		err:   nil,
	}
}}

func (m *BasicSessionMap) NewSession(msg *PduMsg) Session {
	session := sessionpool.Get().(*BasicSession)
	seq := atomic.AddUint32(&m.seq, 1)
	session.seq = seq
	session.begintime = time.Now()
	session.Owner = m
	if session != nil {
		m.lock.Lock()
		m.sessions[seq] = session
		m.lock.Unlock()
		msg.Header.SetSeq(seq)
		return session
	}
	return nil
}

func (m *BasicSessionMap) EndSession(msg *PduMsg) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	seq := msg.Header.Seq()

	session, ok := m.sessions[seq]

	if ok {

		//		delete(m.sessions, seq)
		//		bs := session.(*BasicSession)
		//		bs.reply = *msg
		//		bs.event <- struct{}{}
		session._reply(msg, nil)
		return nil
	}

	return errors.New("session no found")
}

type BasicSession struct {
	seq       uint32
	event     chan struct{}
	reply     PduMsg
	err       error
	begintime time.Time
	Owner     *BasicSessionMap
}

func (s *BasicSession) Wait(timeout time.Duration) (msg *PduMsg, err error) {
	for {
		select {
		case _, ok := <-s.event:
			if !ok {
				return nil, s.err
			}
			return &s.reply, nil
		case <-time.After(timeout):
			s.Owner.lock.Lock()
			delete(s.Owner.sessions, s.seq)
			s.Owner.lock.Unlock()
			return nil, errors.New("timeout")
		}
	}
}

func (s *BasicSession) Close() {
	//close(s.event)
	s.err = nil
	s.seq = 0
	s.Owner = nil
	sessionpool.Put(s)
}

func (s *BasicSession) _reply(msg *PduMsg, err error) {

	defer func() {
		s.Owner.lock.Lock()
		delete(s.Owner.sessions, s.seq)
		s.Owner.lock.Unlock()
	}()

	if err != nil {
		s.err = err
		s.event <- struct{}{}
		return
	}
	s.reply = *msg
	s.event <- struct{}{}
}
