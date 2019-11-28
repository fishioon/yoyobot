package wxbiz

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
)

const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type Wxbiz struct {
	aesBlock cipher.Block
	aesIV    []byte
	token    string
}

type MsgRecv struct {
	Tousername string `xml:"ToUserName"`
	Encrypt    string `xml:"Encrypt"`
	Agentid    string `xml:"AgentID"`
}

type CDATA struct {
	Value string `xml:",cdata"`
}

type MsgSend struct {
	XMLName   xml.Name `xml:"xml"`
	Encrypt   CDATA    `xml:"Encrypt"`
	Signature CDATA    `xml:"MsgSignature"`
	Nonce     CDATA    `xml:"Nonce"`
	Timestamp string   `xml:"TimeStamp"`
}

type MsgFrom struct {
	UserId string
	Name   string
	Alias  string
}

type Msg struct {
	From    MsgFrom
	MsgType string
	MsgId   string
	Text    struct {
		Content string
	}
}

type MsgReply struct {
	XMLName xml.Name `xml:"xml"`
	MsgType string
	Text    struct {
		Content CDATA
	}
}

func New(aeskey, token string) (*Wxbiz, error) {
	key, err := base64.StdEncoding.DecodeString(aeskey + "=")
	if err != nil {
		return nil, errors.New("invalid aes key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	s := &Wxbiz{
		aesBlock: block,
		aesIV:    key[:aes.BlockSize],
		token:    token,
	}
	return s, nil
}

func (s *Wxbiz) VerifyURL(sign, ts, nonce, echostr string) (string, error) {
	msg, err := s.DecryptMsg(sign, ts, nonce, echostr)
	if err != nil {
		return "", err
	}
	return msg.Msg, nil
}

func (s *Wxbiz) UnpackMsg(sign, ts, nonce string, data []byte) (*Msg, error) {
	msgRecv := new(MsgRecv)
	if err := xml.Unmarshal(data, msgRecv); err != nil {
		return nil, err
	}
	plainMsg, err := s.DecryptMsg(sign, ts, nonce, string(msgRecv.Encrypt))
	if err != nil {
		return nil, err
	}
	log.Printf("%+v", plainMsg.Msg)
	m := new(Msg)
	if err = xml.Unmarshal([]byte(plainMsg.Msg), m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Wxbiz) PackMsg(msg *MsgReply, ts, nonce string) (*MsgSend, []byte, error) {
	data, err := xml.Marshal(msg)
	if err != nil {
		return nil, nil, err
	}
	res, err := s.EncryptMsg(ts, nonce, string(data))
	if err != nil {
		return nil, nil, err
	}
	sign := signature(s.token, ts, nonce, res)
	msgSend := &MsgSend{
		Encrypt:   CDATA{res},
		Signature: CDATA{sign},
		Timestamp: ts,
		Nonce:     CDATA{nonce},
	}
	data, err = xml.Marshal(msgSend)
	return msgSend, data, err
}

func (s *Wxbiz) ReplyText(msg *Msg, content string, mention []string) *MsgReply {
	reply := &MsgReply{
		MsgType: "text",
	}
	reply.Text.Content.Value = content
	return reply
}

func (s *Wxbiz) DecryptMsg(sign, ts, nonce, data string) (*PlainMsg, error) {
	if !s.checkSign(sign, ts, nonce, data) {
		return nil, errors.New("signature invalid")
	}

	plaintext, err := s.AesDecrypt(data)
	if err != nil {
		return nil, err
	}

	return s.ParsePlainText(plaintext)
}

func (s *Wxbiz) EncryptMsg(ts, nonce, data string) (string, error) {
	var buffer bytes.Buffer
	buffer.WriteString(randString(16))

	mlen := make([]byte, 4)
	binary.BigEndian.PutUint32(mlen, uint32(len(data)))
	buffer.Write(mlen)
	buffer.WriteString(data)
	buffer.WriteString("")
	return s.AesEncrypt(buffer.Bytes())
}

func (s *Wxbiz) AesEncrypt(data []byte) (string, error) {
	ciphertext, err := aesEncrypt(s.aesBlock, s.aesIV, data)
	if err != nil {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *Wxbiz) AesDecrypt(base64Text string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(base64Text)
	if err != nil {
		return nil, err
	}
	if ciphertext, err = aesDecrypt(s.aesBlock, s.aesIV, ciphertext); err != nil {
		return nil, err
	}
	textLen := len(ciphertext)
	padding := int(ciphertext[textLen-1])
	return ciphertext[:textLen-padding], nil
}

type PlainMsg struct {
	Random    string
	MsgLen    uint32
	Msg       string
	ReceiveID string
}

func (s *Wxbiz) ParsePlainText(plaintext []byte) (*PlainMsg, error) {
	random := plaintext[:16]
	mlen := binary.BigEndian.Uint32(plaintext[16:20])
	if uint32(len(plaintext)) < mlen+20 {
		return nil, fmt.Errorf("invalid plaintext=%s", string(plaintext))
	}
	msg := plaintext[20 : 20+mlen]
	rid := plaintext[20+mlen:]
	return &PlainMsg{
		Random:    string(random),
		MsgLen:    mlen,
		Msg:       string(msg),
		ReceiveID: string(rid),
	}, nil
}

func (s *Wxbiz) checkSign(sign, ts, nonce, data string) bool {
	return signature(s.token, ts, nonce, data) == sign
}

func aesDecrypt(block cipher.Block, iv, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < aes.BlockSize || len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext too short or not a multiple of the block size")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)
	return ciphertext, nil
}

func aesEncrypt(block cipher.Block, iv, plaintext []byte) ([]byte, error) {
	padding := 32 - (len(plaintext) % 32)
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	plaintext = append(plaintext, padtext...)

	ciphertext := make([]byte, len(plaintext))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)
	return ciphertext, nil
}

func signature(token, timestamp, nonce, data string) string {
	fields := []string{token, timestamp, nonce, data}
	sort.Strings(fields)
	s := sha1.Sum([]byte(strings.Join(fields, "")))
	return hex.EncodeToString(s[:])
}

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}
