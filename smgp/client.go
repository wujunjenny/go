package smgp

import (
	//	"bufio"
	//"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Userdata interface{}

type ClientOption struct {
	Net        string
	RemoteAddr string
	Bind_Type  uint32
	Data       Userdata
}

type Client struct {
	option *ClientOption
	conn   *Connector
	log    func(args ...interface{})

	event Eventhandle
	//in      chan SmppMsg
	//out     MChan
	//seq     uint32
	//waitRsp map[uint32]chan SmppMsg
	funcmap map[uint32]MsgHandle
	maplock sync.RWMutex
}

func (cli *Client) SetEventHandle(e Eventhandle) {
	cli.event = e
}

//type MChan struct {
//	ch       chan SmppMsg
//	lock     sync.Mutex
//	ref      int
//	rcvclose bool
//}

//func (ch *MChan) GetChan() chan SmppMsg {
//	ch.lock.Lock()
//	defer ch.lock.Unlock()
//	if ch.rcvclose {
//		return nil
//	}
//	ch.ref++
//	return ch.ch
//}

//func (ch *MChan) Close() {
//	ch.lock.Lock()
//	defer ch.lock.Unlock()
//	if ch.ref == 0 {
//		return
//	}
//	ch.ref--
//	if ch.ref == 0 && ch.rcvclose {
//		close(ch.ch)
//		return
//	}
//}

//func (ch *MChan) CloseRcv() {
//	ch.lock.Lock()
//	defer ch.lock.Unlock()
//	ch.rcvclose = true
//	if ch.ref == 0 {
//		close(ch.ch)
//	}
//}

//func (ch *MChan) Init() {
//	ch.ch = make(chan SmppMsg)
//	ch.ref = 1
//	ch.rcvclose = false
//}

//type Messagehandle func(client *Client, header *SmppHeader, body *SmppBody) (err error)

func NewClient(option *ClientOption) *Client {
	cli := Client{option: option}
	return &cli
}

func (cli *Client) SetLogHandle(f func(args ...interface{})) {
	cli.log = f
}

//func (cli *Client) Handle(cn *Connector, msg *PduMsg) error {
//	cmd := msg.Header.GetCommandID()

//	cli.maplock.RLock()
//	defer cli.maplock.RUnlock()
//	f, ok := cli.funcmap[cmd]
//	if !ok {

//		Queue(func() { cli.UnknownMsgHandle(cn, msg) })
//		return ERROR_NOFINDHANDLE
//	}

//	Queue(func() { f.Handle(cn, msg) })

//	return nil

//}

//func (cli *Client) RegHandleFunc(cmd uint32, handle MsgHandle) {
//	cli.maplock.Lock()
//	defer cli.maplock.Unlock()

//	cli.funcmap[cmd] = handle

//}

func (cli *Client) Open() error {
	if len(cli.option.Net) == 0 {
		cli.option.Net = "tcp"
	}
	pTCPAddr, err := net.ResolveTCPAddr(cli.option.Net, cli.option.RemoteAddr)
	if err != nil {
		return err
	}
	cli.Log("ResolveTCPAddr ok")

	var conn *net.TCPConn

	for retry := 0; retry < 3; retry++ {
		conn, err = net.DialTCP(cli.option.Net, nil, pTCPAddr)

		if err == nil {
			break
		}
		time.Sleep(time.Millisecond * 200)
	}

	if err != nil {
		return err
	}

	cli.Log("DialTCP ok")

	cli.conn = NewConnector(conn)
	cli.conn.SetLogHandle(cli.log)
	//cli.conn.SetOwner(cli)

	cli.conn.onevent = func(c *Connector, t int) int {

		if cli.event != nil {
			return cli.event(c, t)
		}
		return 0
	}

	cli.conn.Start()

	cli.conn.emitConnect()

	//defer func() {
	//	cli.conn.Close()
	//	cli.conn = nil
	//}()

	//	atomic.StoreUint32(&cli.seq, 0)

	//	var header SmppHeader

	//	body := NewBind(&cli.option.Smpp_bind)
	//	header.SetPduLen(uint32(len(body.Data())))
	//	header.SetCommandID(cli.option.Bind_Type)
	//	//header.SetSeq(cli.NewSeq())

	//	rsp, err := cli.conn.sendRequst(SmppMsg{Header: header, Body: *body}, time.Second*2)

	//	//	_, err = conn.Write(header.Data())

	//	if err != nil {
	//		cli.Log("bind erro ", err)
	//		cli.conn.CloseError(err)
	//		return err
	//	}

	//	_, err = conn.Write(body.Data())
	//	if err != nil {
	//		return err
	//	}

	//	var resp_hd SmppHeader
	//	_, err = resp_hd.ReadFromIO(conn)
	//	if err != nil {
	//		return err
	//	}

	//	var rsp_body SmppBody
	//	_, err = rsp_body.ReadFromIO(conn, int(resp_hd.GetPduLen()))

	//	if err != nil {
	//		return err
	//	}

	//	if rsp.Header.GetStatus() != 0 {
	//		cli.conn.CloseError(fmt.Errorf("Bind response error %x", rsp.Header.GetStatus()))
	//		return fmt.Errorf("Bind response error %x", rsp.Header.GetStatus())
	//	}

	atomic.StoreInt32(&cli.conn.status, CONN_ACTIVE)
	cli.Log("Bind ok")
	//cli.in = make(chan SmppMsg)
	//cli.out.Init()

	//go cli.rcv_proc()
	//go cli.snd_proc()

	return nil
}

func (cl *Client) RegHandleFunc(cmd uint32, f MsgFunc) {

	cl.RegHandle(cmd, MsgHandle(f))
}

func (cl *Client) RegHandle(cmd uint32, handle MsgHandle) {

	cl.maplock.Lock()
	defer cl.maplock.Unlock()

	//	switch CMDId(cmd) {
	//	case BIND_RECEIVER, BIND_TRANSMITTER, BIND_TRANSCEIVER:
	//		auth := func() {

	//		}
	//	}

	cl.funcmap[cmd] = handle

}

//func (cl *Client) UnknownMsgHandle(cn *Connector, msg *PduMsg) error {

//	cn.ReplyfromMsg(msg.Header, KNOWN_MSG_TYPE, SmppBody{})
//	return nil
//}

//func (cl *Client) UnknownRespMsgHandle(cn *Connector, msg *PduMsg) error {
//	return nil
//}

//func (cli *Client) send(header SmppHeader, body SmppBody) {
//	msg := SmppMsg{header, body}
//	//sync.Pool.Get().(*SmppMsg)
//	//atomic.CompareAndSwapInt32()
//	ch := cli.out.GetChan()
//	if ch != nil {
//		defer cli.out.Close()
//		ch <- msg
//	}
//}

//func (cli *Client) NewSeq() uint32 {
//return atomic.AddUint32(&cli.seq, 1)
//}

/*func (cli *Client) rcv_proc() {
	defer close(cli.in)
	for {
		var header SmppHeader
		_, err := header.ReadFromIO(cli.conn)
		if err != nil {
			return
		}
		var body SmppBody
		_, err = body.ReadFromIO(cli.conn, int(header.GetPduLen()))
		if err != nil {
			return
		}

		cmd := header.GetCommandID()
		if IsReqCommand(cmd) == false {
			rspchan, ok := cli.waitRsp[header.GetSeq()]
			if ok {
				rspchan <- SmppMsg{header, body}
				close(rspchan)
				delete(cli.waitRsp, header.GetSeq())
				continue
			}
		}

		cli.in <- SmppMsg{header, body}
	}

}*/

/*func (cli *Client) snd_proc() {

	ch := cli.out.ch
	defer cli.out.CloseRcv()
	writer := bufio.NewWriter(cli.conn)
	for {
		select {

		case msg := <-ch:
			_, err := writer.Write(msg.Header.Data())
			if err != nil {
				return
			}
			_, err = writer.Write(msg.Body.Data())
			if err != nil {
				return
			}

		}

	}
}*/
func (cli *Client) GetConn() (*Connector, error) {
	if cli.conn == nil {
		return nil, fmt.Errorf("no connector")
	}

	if cli.conn.lasterror != nil {
		return nil, cli.conn.lasterror
	}

	return cli.conn, nil
}

func (cli *Client) Log(args ...interface{}) {
	if cli.log != nil {
		cli.log(args...)
	}
}

type option func(cn *Connector)

func Dial(options ...option) (cn *Connector, err error) {

	return nil, fmt.Errorf("error connect")
}

func Addr(addr string) option {
	f := func(cn *Connector) {

	}

	return f
}

//config
// addr
// login option
// mux    unknown handle
// log
// default sessionmap  unknown reply
// cn event
