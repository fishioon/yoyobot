package wxbiz

import (
	"log"
	"testing"
)

var (
	// 企业微信官方测试 key 和token
	token  = "QDG6eK"
	aeskey = "jWmYm7qr5nMoAUwZRjGtBxmz3KA1tkAj3ykkR6q2B2C"
)

func TestVerifyURL(t *testing.T) {
	wx, _ := New(aeskey, token)

	echostr := "P9nAzCzyDtyTWESHep1vC5X9xho/qYX3Zpb4yKa9SKld1DsH3Iyt3tP3zNdtp+4RPcs8TgAE7OaBO+FZXvnaqQ=="
	sign := "5c45ff5e21c57e6ad56bac8758b79b1d9ac89fd3"
	ts := "1409659589"
	nonce := "263014780"
	urlMsg := "1616140317555161061"
	res, _ := wx.VerifyURL(sign, ts, nonce, echostr)
	if res != urlMsg {
		t.Errorf("url msg expect %s got %s", urlMsg, res)
	}
}

func TestCrytpMsg(t *testing.T) {
	wx, _ := New(aeskey, token)

	reqTimestamp := "1409659813"
	reqNonce := "1372623149"

	msg1 := &MsgReply{
		MsgType: "text",
	}
	msg1.Text.Content.Value = "this is a text"
	m, msgtext, err := wx.PackMsg(msg1, reqTimestamp, reqNonce)
	if err != nil {
		t.Error("packmsg", err)
	}
	sign := m.Signature.Value
	msg2, err := wx.UnpackMsg(sign, reqTimestamp, reqNonce, msgtext)
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("%+v", msg2)
	if msg2.Text.Content != msg1.Text.Content.Value {
		t.Errorf("msg content expect %s got %s", msg2.Text.Content, msg1.Text.Content.Value)
	}
}
