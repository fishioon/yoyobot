package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"git.code.oa.com/ifishjin/yoyobot/wxbiz"
	"github.com/go-redis/redis"
)

var (
	ErrSystemError         = NewError(1, "system error")
	ErrBadCommand          = NewError(2, "bad command")
	ErrInvalidDays         = NewError(3, "bad days")
	ErrInsufficientBalance = NewError(4, "insufficient balance")
)

type YoErr struct {
	Code    int
	Message string
}

func (e *YoErr) Error() string {
	return fmt.Sprintf("%d %s", e.Code, e.Message)
}

func NewError(code int, message string) *YoErr {
	return &YoErr{code, message}
}

type server struct {
	mux *http.ServeMux
	wx  *wxbiz.Wxbiz
	db  *redis.Client
}

func newServer() (*server, error) {
	client := redis.NewClient(&conf.Redis)
	if err := client.Ping().Err(); err != nil {
		return nil, err
	}
	wx, err := wxbiz.New(conf.AesKey, conf.Token)
	if err != nil {
		return nil, err
	}
	s := &server{
		mux: http.NewServeMux(),
		wx:  wx,
		db:  client,
	}

	s.init()
	return s, nil
}

type response struct {
	Errors string      `json:"errors"`
	Status int         `json:"status"`
	Data   interface{} `json:"data"`
}

func (s *server) init() {
	s.mux.HandleFunc("/yoyobot/listen", s.handleListen)
	s.mux.HandleFunc("/admin/set", s.handleAdminSetBalance)
	s.mux.HandleFunc("/admin/show", s.handleShow)
	/*
		staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./static")))
		s.mux.Handle("/static/", staticHandler)
	*/
}
func (s *server) handleListen(w http.ResponseWriter, r *http.Request) {
	log.Printf("request: %+v", r.URL.Query())
	sign := r.URL.Query().Get("msg_signature")
	ts := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")
	if r.Method == "GET" {
		// check url
		echostr := r.URL.Query().Get("echostr")
		msg, err := s.wx.VerifyURL(sign, ts, nonce, echostr)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		w.Write([]byte(msg))
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "can't read body", http.StatusBadRequest)
		return
	}
	msg, err := s.wx.UnpackMsg(sign, ts, nonce, body)
	if err != nil {
		log.Printf("Error parse msg: %v", err)
		http.Error(w, "invalid msg", http.StatusBadRequest)
		return
	}
	replyMsg, err := s.handleMsg(msg)
	if err != nil {
		log.Printf("handle msg error: %s content=%s", err.Error(), msg.Text.Content)
		replyMsg = s.wx.ReplyText(msg, "yoyo 没懂您的意思，示例：休假/加班 0.5天 20191111上午", nil)
	}
	_, res, err := s.wx.PackMsg(replyMsg, ts, nonce)
	if err != nil {
		log.Printf("Error pack msg: %v", err)
		http.Error(w, "pack msg error", http.StatusBadRequest)
		return
	}
	log.Printf("reply: %s", res)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(200)
	w.Write(res)
}

func (s *server) handleAdminSetBalance(w http.ResponseWriter, r *http.Request) {
	// TODO: admin check
	req := make(map[string]interface{})
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error pack msg: %v", err)
		http.Error(w, "pack msg error", http.StatusBadRequest)
		return
	}
	days := make(map[string]int64)
	for k, v := range req {
		if a, ok := v.(float64); ok {
			days[k] = int64(a * 2)
		}
	}

	log.Printf("days: %+v", days)
	if err := DaySet(s.db, days); err != nil {
		log.Printf("dayset: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("{}"))
}

func (s *server) handleShow(w http.ResponseWriter, r *http.Request) {
	balance, err := DayShow(s.db)
	if err != nil {
		log.Printf("dayshow: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res, _ := json.MarshalIndent(balance, "", "\t")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write(res)
}

func (s *server) handleMsg(msg *wxbiz.Msg) (reply *wxbiz.MsgReply, err error) {
	log.Printf("from: %s msg: %s", msg.From.Alias, msg.Text.Content)
	defer func() {
		if err != nil {
			content := err.Error()
			if e, ok := err.(*YoErr); ok {
				if e.Code <= len(conf.Errmsg) && e.Code > 0 {
					content = conf.Errmsg[e.Code-1]
				}
			}
			reply = s.wx.ReplyText(msg, content, nil)
			err = nil
		}
	}()
	arr := strings.Fields(msg.Text.Content)
	if len(arr) < 2 {
		return nil, ErrBadCommand
	}
	cmd := arr[1]
	if cmd == "我的假期" {
		balance, err := DayGet(s.db, msg.From.Alias)
		if err != nil {
			return nil, err
		}
		return s.wx.ReplyText(msg, fmt.Sprintf("%s 余额 %s 天", msg.From.Alias, FormatAmount(balance)), nil), err
	}

	if len(arr) < 4 {
		return nil, ErrBadCommand
	}
	adds, err := ParseAmount(arr[2])
	if err != nil || adds <= 0 {
		return nil, ErrInvalidDays
	}
	if cmd == "休假" {
		adds = -adds
	} else if cmd == "加班" {
	} else {
		return nil, ErrBadCommand
	}

	balance, err := DayAdd(s.db, msg.From.Alias, adds)
	if err != nil {
		return nil, err
	}
	content := fmt.Sprintf("%s %s 成功，已经增加 %s 天, 目前余额: %s", msg.From.Alias, cmd, FormatAmount(adds), FormatAmount(balance))
	reply = s.wx.ReplyText(msg, content, nil)
	return reply, err
}

func (s *server) handleLeave(w http.ResponseWriter, r *http.Request) {
	req := &struct {
		Image []byte `json:"image"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		log.Printf("error decoding request: %v", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	res := &struct {
	}{}
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Printf("error encoding response: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	/*
		if r.Header.Get("X-Forwarded-Proto") == "http" {
			r.URL.Scheme = "https"
			r.URL.Host = r.Host
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
			return
		}
		if r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; preload")
		}
	*/
	s.mux.ServeHTTP(w, r)
}
