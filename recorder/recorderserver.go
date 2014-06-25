package recorder

import (
	"code.google.com/p/go.net/websocket"
	"saassoft.net/signaldistribution/base"
	"saassoft.net/signaldistribution/signal"
	"strings"
)

type SignalCache struct {
	ChannelSignals map[string][]*signal.Signal
	Signals        map[string]bool
}

type Station struct {
	IpAddr string
	Conn   *websocket.Conn
	Info   *base.ServerInfo
}

type RecorderServer struct {
	Stations    map[string]*Station
	SignalCache *SignalCache
	Info        *base.ServerInfo
	Token       string
}

func (this *RecorderServer) InitWith(info *base.ServerInfo, token string) {
	this.Info = info
	this.Token = token
	this.Stations = make(map[string]*Station)
	this.SignalCache = &SignalCache{ChannelSignals: make(map[string][]*signal.Signal), Signals: make(map[string]bool)}
}

func (this *RecorderServer) StationJoin(ws *websocket.Conn) {

	remoteInfo, err := base.SwitchServerInfo(ws, this.Info)
	if err != nil {
		return
	}
	remoteAddr := remoteInfo.Addr()
	this.Stations[remoteAddr] = &Station{
		IpAddr: remoteAddr,
		Conn:   ws,
		Info:   remoteInfo,
	}
	this.doRecord(ws)
	this.Stations[remoteAddr] = nil
	delete(this.Stations, remoteAddr)
}

func (this *Station) switchInfo(ws *websocket.Conn) (*base.ServerInfo, error) {
	if err := websocket.JSON.Send(ws, this.Info); err != nil {
		return nil, err
	}
	var remoteInfo base.ServerInfo
	if err := websocket.JSON.Receive(ws, &remoteInfo); err != nil {
		return nil, err
	}
	if remoteInfo.IP == "" && ws.IsServerConn() {
		remoteAddr := ws.Request().RemoteAddr
		remoteInfo.IP = base.SubString(remoteAddr, 0, strings.Index(remoteAddr, ":"))
	}
	if err := websocket.JSON.Send(ws, remoteInfo); err != nil {
		return nil, err
	}
	var localInfo base.ServerInfo
	if err := websocket.JSON.Receive(ws, &localInfo); err != nil {
		return nil, err
	}
	if this.Info.IP == "" && localInfo.IP != "" {
		this.Info.IP = localInfo.IP
	}
	return &remoteInfo, nil
}

func (this *RecorderServer) Fetch(ws *websocket.Conn) {
	var channelid string
	var token string
	var lastfrom string
	signals := []*signal.Signal{}
	request := ws.Request()
	request.ParseForm()
	if channelid = request.Form.Get("cid"); channelid == "" {
		signals = append(signals, this.newError("no cid"))
		websocket.JSON.Send(ws, signals)
		return
	}

	if token = request.Form.Get("token"); token == "" {
		signals = append(signals, this.newError("no token"))
		websocket.JSON.Send(ws, signals)
		return
	}
	channelSignas := this.SignalCache.ChannelSignals[channelid]
	if channelSignas == nil {
		websocket.JSON.Send(ws, signals)
		return
	}
	lastfrom = request.Form.Get("lastfrom")
	idx := 0
	signalCount := len(channelSignas)
	if lastfrom != "" {
		for idx = signalCount - 1; idx >= 0; idx-- {
			if strings.HasPrefix(channelSignas[idx].Text, lastfrom) {
				break
			}
		}
	}
	if idx >= 0 {
		for i := idx; i < signalCount; i++ {
			signals = append(signals, channelSignas[i])
		}
	}
	websocket.JSON.Send(ws, signals)
}

func (this *RecorderServer) doRecord(ws *websocket.Conn) {
	for {
		var signalPack signal.SignalPack
		if err := websocket.JSON.Receive(ws, &signalPack); err != nil {
			return
		}
		if signalPack.Signal.Type != base.SIGNALTYPE_BLANK {
			if this.SignalCache.Signals[signalPack.Signal.ID] {
				continue
			}
			this.SignalCache.Signals[signalPack.Signal.ID] = true
			if this.SignalCache.ChannelSignals[signalPack.CID] == nil {
				this.SignalCache.ChannelSignals[signalPack.CID] = []*signal.Signal{}
			}
			this.SignalCache.ChannelSignals[signalPack.CID] = append(this.SignalCache.ChannelSignals[signalPack.CID], &signalPack.Signal)
		}
	}
}

func (this *RecorderServer) newError(text string) *signal.Signal {
	signal := &signal.Signal{Type: base.SIGNALTYPE_ERROR, Text: text}
	return signal
}
