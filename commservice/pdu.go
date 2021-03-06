// pdu
package commservice

import (
	"bytes"
	"encoding/binary"
	"errors"
	//"fmt"
	"io"

	"github.com/djimenez/iconv-go"
)

type CMDId uint32

const (
	// PDU Types
	CLIENT_BIND_REC       CMDId = 0x00000000
	CLIENT_BIND_REC_RESP  CMDId = 0x00008000
	CLIENT_BIND           CMDId = 0x00000001
	CLIENT_BIND_RESP      CMDId = 0x00008001
	BIND_TRANSMITTER      CMDId = 0x00000002
	BIND_TRANSMITTER_RESP CMDId = 0x80000002
	QUERY_SM              CMDId = 0x00000003
	QUERY_SM_RESP         CMDId = 0x80000003
	SUBMIT_SM             CMDId = 0x00000004
	SUBMIT_SM_RESP        CMDId = 0x80000004
	DELIVER_SM            CMDId = 0x00000005
	DELIVER_SM_RESP       CMDId = 0x80000005
	UNBIND                CMDId = 0x00000006
	UNBIND_RESP           CMDId = 0x80000006
	REPLACE_SM            CMDId = 0x00000007
	REPLACE_SM_RESP       CMDId = 0x80000007
	CANCEL_SM             CMDId = 0x00000008
	CANCEL_SM_RESP        CMDId = 0x80000008
	BIND_TRANSCEIVER      CMDId = 0x00000009
	BIND_TRANSCEIVER_RESP CMDId = 0x80000009
	OUTBIND               CMDId = 0x0000000B
	ENQUIRE_LINK          CMDId = 0x00000015
	ENQUIRE_LINK_RESP     CMDId = 0x80000015
	SUBMIT_MULTI          CMDId = 0x00000021
	SUBMIT_MULTI_RESP     CMDId = 0x80000021
	ALERT_NOTIFICATION    CMDId = 0x00000102
	DATA_SM               CMDId = 0x00000103
	DATA_SM_RESP          CMDId = 0x80000103
)

func IsReqCommand(cmd uint32) bool {
	return cmd&0x8000 == 0
}

type PduHead interface {
	Data() []byte
	GetHeadLen() int32
	GetPduLen() uint32
	SetPduLen(len uint32)
	//	CommandID() uint32
	//	Seq() uint32
	ReadFromIO(reader *io.Reader)
}

type PduBody interface {
	Data() []byte
	ReadFromIO(reader *io.Reader, sz int)
}

type Header struct {
	//PduHead
	data [20]byte
}

type Body struct {
	//PduBody
	data []byte
}

type CommMsg struct {
	Header Header
	Body   Body
}

func (h *Header) Data() []byte {

	return h.data[:]
}

func (h *Header) GetHeadLen() int32 {

	return 20
}

func (h *Header) GetPduLen() uint32 {

	return binary.LittleEndian.Uint32(h.data[:4]) - uint32(h.GetHeadLen())

}

func (h *Header) SetPduLen(len uint32) {

	binary.LittleEndian.PutUint32(h.data[:4], len+uint32(h.GetHeadLen()))

}

func (h *Header) ReadFromIO(reader io.Reader) (sz int, err error) {

	return io.ReadFull(reader, h.data[:20])
}

func (h *Header) Read(rdata []byte) (sz int, err error) {

	if len(rdata) < 20 {
		return 0, errors.New("input len < min header")
	}

	copy(h.data[:], rdata[:20])
	return 20, nil
}

func (h *Header) GetCommandID() uint32 {

	return binary.LittleEndian.Uint32(h.data[4:])
}

func (h *Header) SetCommandID(id uint32) {

	binary.LittleEndian.PutUint32(h.data[4:], id)
}

func (h *Header) GetStatus() uint32 {

	return binary.LittleEndian.Uint32(h.data[8:])
}

func (h *Header) SetStatus(id uint32) {

	binary.LittleEndian.PutUint32(h.data[8:], id)
}

func (h *Header) GetSeq() uint32 {

	return binary.LittleEndian.Uint32(h.data[12:])
}

func (h *Header) SetSeq(id uint32) {

	binary.LittleEndian.PutUint32(h.data[12:], id)
}

func (body *Body) Data() []byte {

	return body.data
}

func (body *Body) SetData(d []byte) {
	body.data = d
}

func (body *Body) ReadFromIO(reader io.Reader, sz int) (rz int, err error) {

	if sz > cap(body.data) {
		body.data = make([]byte, sz)
	} else {
		body.data = body.data[:sz]
	}
	return io.ReadFull(reader, body.data[:sz])
}

func smpp_writerstring(writer io.Writer, v string) (sz int, err error) {
	var buf bytes.Buffer
	buf.WriteString(v)
	buf.WriteByte(0)
	return writer.Write(buf.Bytes())
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
	return string(tlv.Value)
}

func (tlv *TLV) SetString(v string) {

	tlv.Value = []byte(v)
}

func (tlv *TLV) UInt32() uint32 {
	return binary.BigEndian.Uint32(tlv.Value)
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

type Comm_bind struct {
	Name     string
	Type     int
	Password string
}

type Comm_bind_resp struct {
	System_id string
	AuthData  []byte
}

type Smpp_submit_sm struct {
	Service_type            string
	Src_ton                 byte
	Src_npi                 byte
	Src_Addr                string
	Dst_ton                 byte
	Dst_npi                 byte
	Dst_Addr                string
	Ems_class               byte
	Protocol_id             byte
	Pri                     byte
	Schedule_delivery_time  string
	Validity_period         string
	Registered_delivery     byte
	Replace_if_present_flag byte
	DCS                     byte
	Sm_default_id           byte
	Sm_len                  byte
	Short_message           []byte
}

type Smpp_submit_sm_resp struct {
	Sm_id string
}

func NewBind(arg *Comm_bind, opt ...TLV) *Body {
	rt := &Body{}

	var bf bytes.Buffer

	var tmp [50]byte
	copy(tmp[:], arg.Name)
	bf.Write(tmp[:16])

	binary.LittleEndian.PutUint32(tmp[:4], uint32(arg.Type))
	bf.Write(tmp[:4])

	copy(tmp[:], arg.Password)
	bf.Write(tmp[:9])

	rt.data = bf.Bytes()
	return rt
}

func NewBindResp(arg *Comm_bind_resp, opt ...TLV) *Body {
	rt := &Body{}
	var bf bytes.Buffer
	var tmp [50]byte
	copy(tmp[:], arg.System_id)
	bf.Write(tmp[:16])
	copy(tmp[:], arg.AuthData)
	bf.Write(tmp[:16])
	for _, v := range opt {
		v.Serialize(&bf)
	}
	rt.data = bf.Bytes()

	return rt
}

func NewSubmitSM(arg *Smpp_submit_sm, opt ...TLV) *Body {
	rt := &Body{}

	var bf bytes.Buffer
	bf.Grow(2048)
	bf.WriteString(arg.Service_type)
	bf.WriteByte(0)
	bf.WriteByte(arg.Src_ton)
	bf.WriteByte(arg.Src_npi)
	bf.WriteString(arg.Src_Addr)
	bf.WriteByte(0)
	bf.WriteByte(arg.Dst_ton)
	bf.WriteByte(arg.Dst_npi)
	bf.WriteString(arg.Dst_Addr)
	bf.WriteByte(0)
	bf.WriteByte(arg.Ems_class)
	bf.WriteByte(arg.Protocol_id)
	bf.WriteByte(arg.Pri)
	bf.WriteString(arg.Schedule_delivery_time)
	bf.WriteByte(0)
	bf.WriteString(arg.Validity_period)
	bf.WriteByte(0)
	bf.WriteByte(arg.Registered_delivery)
	bf.WriteByte(arg.Replace_if_present_flag)
	bf.WriteByte(arg.DCS)
	bf.WriteByte(arg.Sm_default_id)
	//bf.WriteByte(arg.Sm_len)
	bf.WriteByte(byte(len(arg.Short_message)))
	bf.Write(arg.Short_message)
	for _, v := range opt {
		v.Serialize(&bf)
	}
	rt.data = bf.Bytes()
	return rt
}

func NewSubmitSMRep(arg *Smpp_submit_sm_resp, opt ...TLV) *Body {
	rt := &Body{}
	var bf bytes.Buffer
	bf.WriteString(arg.Sm_id)
	bf.WriteByte(0)
	for _, v := range opt {
		v.Serialize(&bf)
	}
	rt.data = bf.Bytes()
	return rt
}

func ParseBind(bd *Body, tlvs *[]TLV) (rt *Comm_bind, err error) {

	bf := bytes.NewBuffer(bd.data)
	pk := &Comm_bind{}
	var tmp [50]byte

	_, err = bf.Read(tmp[:16])
	if err != nil {
		return nil, err
	}
	pk.Name = string(bytes.TrimRight(tmp[:], "\x00"))
	_, err = bf.Read(tmp[:4])
	if err != nil {
		return nil, err
	}
	pk.Type = int(binary.LittleEndian.Uint32(tmp[:]))

	_, err = bf.Read(tmp[:9])
	pk.Password = string(bytes.TrimRight(tmp[:], "\x00"))
	rt = pk
	return
}

func ParseBindResp(bd *Body, tlvs *[]TLV) (rt *Comm_bind_resp, err error) {
	bf := bytes.NewBuffer(bd.data)
	pk := &Comm_bind_resp{}
	var ss []byte
	ss, err = bf.ReadBytes(0)
	if err != nil {
		return nil, err
	}
	pk.System_id = string(ss)

	if tlvs != nil {
		for {
			tlv, _ := ReadTlv(bf)
			if tlv == nil {
				break
			}
			*tlvs = append(*tlvs, *tlv)
		}

	}

	return pk, nil

}

func ParseSubmitSM(bd *Body, tlvs *[]TLV) (rt *Smpp_submit_sm, err error) {
	bf := bytes.NewBuffer(bd.data)
	pk := &Smpp_submit_sm{}
	var ss []byte
	ss, err = bf.ReadBytes(0)
	if err != nil {
		return nil, err
	}
	pk.Service_type = string(ss)
	pk.Src_ton, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}
	pk.Src_npi, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}
	ss, err = bf.ReadBytes(0)
	if err != nil {
		return nil, err
	}
	pk.Src_Addr = string(ss)

	pk.Dst_ton, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}
	pk.Dst_npi, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}
	ss, err = bf.ReadBytes(0)
	if err != nil {
		return nil, err
	}
	pk.Dst_Addr = string(ss)

	pk.Ems_class, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}
	pk.Protocol_id, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}
	pk.Pri, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}

	ss, err = bf.ReadBytes(0)
	if err != nil {
		return nil, err
	}
	pk.Schedule_delivery_time = string(ss)

	ss, err = bf.ReadBytes(0)
	if err != nil {
		return nil, err
	}
	pk.Validity_period = string(ss)

	pk.Registered_delivery, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}
	pk.Replace_if_present_flag, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}
	pk.DCS, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}
	pk.Sm_default_id, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}
	pk.Sm_len, err = bf.ReadByte()
	if err != nil {
		return nil, err
	}

	pk.Short_message = make([]byte, pk.Sm_len)

	_, err = bf.Read(pk.Short_message)
	if err != nil {
		return nil, err
	}

	if tlvs != nil {
		for {
			tlv, _ := ReadTlv(bf)
			if tlv == nil {
				break
			}
			*tlvs = append(*tlvs, *tlv)
		}

	}

	return pk, nil
}

func ParseSubmitSMResp(bd *Body, tlvs *[]TLV) (rt *Smpp_submit_sm_resp, err error) {
	bf := bytes.NewBuffer(bd.data)
	pk := &Smpp_submit_sm_resp{}
	var ss []byte
	ss, err = bf.ReadBytes(0)
	if err != nil {
		return nil, err
	}
	pk.Sm_id = string(ss)

	if tlvs != nil {
		for {
			tlv, _ := ReadTlv(bf)
			if tlv == nil {
				break
			}
			*tlvs = append(*tlvs, *tlv)
		}

	}

	return pk, nil

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

func (sm *Smpp_submit_sm) parseUD() (hd Udhs, payload []byte) {
	//fmt.Println(sm.Ems_class)
	if IsUdhi(sm.Ems_class) {

		l, h, err := GetUdhs(sm.Short_message)
		//fmt.Println(l, h, err)
		if err != nil {
			return
		}
		hd = h

		payload = sm.Short_message[l:]
		return

	}

	return nil, sm.Short_message
}

func (sm *Smpp_submit_sm) GetContentString() (rt string) {

	_, payload := sm.parseUD()

	if sm.DCS == 15 {
		rt, _ = GetUtfStringFromByes(payload)
		return
	}

	if sm.DCS&0x0C == 8 {
		rt, _ = GetUtfStringFromUCS2(payload)
		return
	}

	rt = string(payload)
	return
}
