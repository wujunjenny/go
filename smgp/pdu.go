// pdu
package smgp

import (
	"bytes"
	"encoding/binary"
	"errors"
	//"fmt"
	"io"

	"github.com/djimenez/iconv-go"
)

type CMDId interface {
	IsReqCommand() bool
	ID() uint32
}

type PduHead interface {
	Data() []byte
	GetHeadLen() uint32
	GetPduLen() uint32
	SetPduLen(len uint32)
	GetCommandID() CMDId
	SetCommandID(id CMDId)
	Seq() uint32
	SetSeq(uint32)
	ReadFromIO(reader io.Reader) (sz int, err error)
}

type PduBody interface {
	Data() []byte
	ReadFromIO(reader io.Reader, size int) (sz int, err error)
}

type PduMsg struct {
	Header PduHead
	Body   PduBody
}

type TLV struct {
	Tag   uint16
	Value []byte
}

func NewTlvString(tag uint16, v string) *TLV {
	tlv := &TLV{}
	tlv.Tag = tag
	tlv.Value = []byte(v)
	return tlv
}

func NewTlvUint32(tag uint16, v uint32) *TLV {
	tlv := &TLV{}
	tlv.Tag = tag
	tlv.Value = make([]byte, 4)
	binary.BigEndian.PutUint32(tlv.Value, v)
	return tlv
}

func NewTlvByte(tag uint16, v byte) *TLV {
	tlv := &TLV{}
	tlv.Tag = tag
	tlv.Value = append(tlv.Value, v)
	return tlv
}

func (tlv *TLV) String() string {
	i := bytes.IndexByte(tlv.Value, 0)
	if i < 0 {
		return string(tlv.Value)
	}
	return string(tlv.Value[:i])
}

func (tlv *TLV) SetString(v string) {

	tlv.Value = []byte(v)
}

func (tlv *TLV) UInt32() uint32 {
	return binary.BigEndian.Uint32(tlv.Value)
}

func (tlv *TLV) Byte() byte {
	if len(tlv.Value) == 1 {
		return tlv.Value[0]
	}

	return 0
}

func (tlv *TLV) SetUInt32(v uint32) {
	if len(tlv.Value) < 4 {
		tlv.Value = make([]byte, 4)
	}
	binary.BigEndian.PutUint32(tlv.Value, v)
	tlv.Value = tlv.Value[:4]
}

func (tlv *TLV) Serialize(writer io.Writer) (sz int, err error) {
	var h [4]byte
	binary.BigEndian.PutUint16(h[:], tlv.Tag)
	binary.BigEndian.PutUint16(h[2:], uint16(len(tlv.Value)))
	writer.Write(h[:])
	return writer.Write(tlv.Value)
}

func (tlv *TLV) UnSerialize(reader io.Reader) (sz int, err error) {
	var h [4]byte
	if sz, err = io.ReadFull(reader, h[:]); err != nil {
		return sz, err
	}
	tlv.Tag = binary.BigEndian.Uint16(h[:])
	l := binary.BigEndian.Uint16(h[2:])

	if int(l) > cap(tlv.Value) {
		tlv.Value = make([]byte, l)
	} else {
		tlv.Value = tlv.Value[:l]
	}

	sz, err = io.ReadFull(reader, tlv.Value)

	return sz + 4, err

}

func ReadTlv(reader io.Reader) (tlv *TLV, err error) {
	tlv = &TLV{}
	if sz, err := tlv.UnSerialize(reader); err != nil {
		if sz > 0 {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, io.EOF
	}
	return tlv, nil
}

type TLVS []TLV

func (vs *TLVS) FindTlv(tag uint16) (tlv *TLV, err error) {
	for k, v := range *vs {

		if v.Tag == tag {
			return &(*vs)[k], nil
		}

	}
	return nil, errors.New("tag no found")
}

func (vs *TLVS) InsertOrReplace(tlv TLV) (TLVS, error) {
	for k, v := range *vs {
		if v.Tag == tlv.Tag {
			(*vs)[k] = tlv
			return *vs, nil
		}
	}

	return append(*vs, tlv), nil
}

func IsUdhi(emclass byte) bool {
	return emclass&0x40 != 0
}

func SetUdhi(emclass byte) byte {
	return emclass | 0x40
}

type Udh struct {
	Tag   byte
	Value []byte
}

type Udhs []Udh

func GetUdhs(input []byte) (l int, rt []Udh, err error) {
	if len(input) < 1 {
		return 0, nil, errors.New("error iput len1")
	}
	headlen := input[0]
	if int(headlen+1) > len(input) {
		return 0, nil, errors.New("error iput len2")
	}
	data := input[1 : headlen+1]
	bf := bytes.NewBuffer(data)
	for {
		var h Udh
		var l byte
		h.Tag, err = bf.ReadByte()
		if err != nil {
			break
		}
		l, err = bf.ReadByte()
		if err != nil {
			return int(headlen) + 1, rt, err
		}
		h.Value = make([]byte, l)
		_, err = bf.Read(h.Value)
		if err != nil {
			return int(headlen) + 1, rt, err
		}
		rt = append(rt, h)
	}
	return int(headlen) + 1, rt, nil
}

func (us *Udhs) FindLongHd() (ref uint8, total uint8, index uint8, err error) {

	for _, v := range *us {
		if v.Tag == 0 {
			if len(v.Value) == 3 {
				return v.Value[0], v.Value[1], v.Value[2], nil
			} else {
				return 0, 0, 0, errors.New("error tag len")
			}
		}

	}
	return 0, 0, 0, errors.New("longsm tag no found")
}

func GetUdhBin(uds Udhs) []byte {
	var bf bytes.Buffer
	for _, v := range uds {
		bf.WriteByte(v.Tag)
		bf.WriteByte(byte(len(v.Value)))
		bf.Write(v.Value)
	}
	var rt []byte
	rt = append(rt, byte(len(bf.Bytes())))
	rt = append(rt, bf.Bytes()...)
	return rt
}

func GetUtfStringFromUCS2(input []byte) (rt string, err error) {
	out := make([]byte, len(input)*2)
	_, ncode, err := iconv.Convert(input, out, "UNICODEBIG", "utf-8")

	return string(out[:ncode]), err
}

func GetUCS2FromUtfString(input string) (rt []byte, err error) {
	out := make([]byte, len(input)*2)
	_, ncode, err := iconv.Convert([]byte(input), out, "utf-8", "UNICODEBIG")

	return out[:ncode], err
}

func GetUtfStringFromByes(input []byte) (rt string, err error) {
	out := make([]byte, len(input)*2)
	_, ncode, err := iconv.Convert(input, out, "gb2312", "utf-8")
	return string(out[:ncode]), err

}

func GetStringFromUtfString(input string) (rt []byte, err error) {
	out := make([]byte, len(input))
	_, ncode, err := iconv.Convert([]byte(input), out, "utf-8", "gb2312")
	return out[:ncode], err
}
