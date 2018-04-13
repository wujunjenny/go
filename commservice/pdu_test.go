// pdu_test.go
package commservice

import (
	"testing"
)

func TestUSCIItoUtf8(t *testing.T) {

	src := []byte{0x4e, 0x2d, 0x56, 0xfd, 0x6c, 0x49, 0x5b, 0x57, 0, 0x30, 0, 0x31}

	dest, err := GetUtfStringFromUCS2(src)

	if err == nil {
		t.Log("ok utf8 txt", dest)
		t.Logf("ok utf8(hex) %X", dest)

	}

	dest2, err := GetUCS2FromUtfString(dest)

	if err == nil {
		t.Logf("ok unicode %X", dest2)
	}

}

func TestGBtoUtf8(t *testing.T) {

	src := []byte{0xd6, 0xd0, 0xbb, 0xaa, 0xc8, 0xcb, 0xc3, 0xf1, 0xb9, 0xb2, 0xba, 0xcd, 0xb9, 0xfa, 0x30, 0x31, 0x31}

	dest, err := GetUtfStringFromByes(src)

	if err == nil {
		t.Log("ok utf8 txt", dest)
		t.Logf("ok utf8(hex) %X", dest)

	}

}

func TestTlv(t *testing.T) {

	var ts = TLVS{}

	ts, _ = ts.InsertOrReplace(TLV{Tag: 1, Value: []byte("1234567")})
	ts, _ = ts.InsertOrReplace(TLV{Tag: 2, Value: []byte("2234567")})
	ts, _ = ts.InsertOrReplace(TLV{Tag: 3, Value: []byte("3234567")})
	ts, _ = ts.InsertOrReplace(TLV{Tag: 4, Value: []byte("42345678")})
	ts, _ = ts.InsertOrReplace(TLV{Tag: 4, Value: []byte("1234567890")})

	t.Log(ts)

	v, _ := ts.FindTlv(1)

	v.Value = []byte("223344")

	t.Log(v)
	t.Log(ts)

}
