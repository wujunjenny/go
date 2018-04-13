// smgp project smgp.go
package smgp

import "io"
import "encoding/binary"
import "errors"
import "bytes"
import "sync"

func IsReqCommand(cmd uint32) bool {
	return cmd&0x80000000 == 0
}

type SmgpHeader struct {
	//PduHead
	data [12]byte
}

type SmgpBody struct {
	//PduBody
	data []byte
}

func (h *SmgpHeader) Data() []byte {

	return h.data[:]
}

func (h *SmgpHeader) GetHeadLen() uint32 {

	return uint32(len(h.data))
}

func (h *SmgpHeader) GetPduLen() uint32 {

	return binary.BigEndian.Uint32(h.data[:4]) - uint32(len(h.data))

}

func (h *SmgpHeader) SetPduLen(sz uint32) {

	binary.BigEndian.PutUint32(h.data[:4], sz+uint32(len(h.data)))

}

func (h *SmgpHeader) ReadFromIO(reader io.Reader) (sz int, err error) {

	return io.ReadFull(reader, h.data[:])
}

func (h *SmgpHeader) Read(rdata []byte) (sz int, err error) {

	if len(rdata) < len(h.data) {
		return 0, errors.New("input len < min header")
	}

	copy(h.data[:], rdata)
	return len(h.data), nil
}

func (h *SmgpHeader) GetCommandID() uint32 {

	return binary.BigEndian.Uint32(h.data[4:])
}

func (h *SmgpHeader) SetCommandID(id uint32) {

	binary.BigEndian.PutUint32(h.data[4:], id)
}

func (h *SmgpHeader) GetSeq() uint32 {

	return binary.BigEndian.Uint32(h.data[8:])
}

func (h *SmgpHeader) SetSeq(id uint32) {

	binary.BigEndian.PutUint32(h.data[8:], id)
}

func (body *SmgpBody) Data() []byte {

	return body.data
}

func (body *SmgpBody) ReadFromIO(reader io.Reader, sz int) (rz int, err error) {

	if sz > cap(body.data) {
		body.data = make([]byte, sz)
	} else {
		body.data = body.data[:sz]
	}
	return io.ReadFull(reader, body.data[:sz])
}

//func smpp_writerstring(writer io.Writer, v string) (sz int, err error) {
//	var buf bytes.Buffer
//	buf.WriteString(v)
//	buf.WriteByte(0)
//	return writer.Write(buf.Bytes())
//}

func WriteFixString(writer io.Writer, v string, fixlen int) (sz int, err error) {
	buf := getfixlenbuf(fixlen)
	defer putfixlenbuf(buf)
	copy(buf, v)
	return writer.Write(buf)
}

func ReadFxiString(reader io.Reader, fixlen int) (s string, err error) {
	buf := getfixlenbuf(fixlen)
	defer putfixlenbuf(buf)
	n, err := reader.Read(buf)
	if err != nil {
		return "", err
	}
	n = bytes.IndexByte(buf, 0)
	if n >= 0 {
		return string(buf[n:]), nil
	}
	return string(buf), nil
}

var fixbuf_pool sync.Pool

func getfixlenbuf(fix int) []byte {

	b := fixbuf_pool.Get().([]byte)

	if b == nil && fix < 1024 {
		b = make([]byte, 1024)
	}

	if cap(b) < fix {

		b = make([]byte, fix*2)
	}

	return b[:fix]
}

func putfixlenbuf(buf []byte) {

	if cap(buf) < 1024 {
		return
	}

	fixbuf_pool.Put(buf)

}
