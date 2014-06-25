// Copyright 2014 liveease.com. All rights reserved.

package signal

import (
	"code.google.com/p/go-uuid/uuid"
	"code.google.com/p/go.net/websocket"
	"errors"
	"log"
	"saassoft.net/signaldistribution/base"
	"strconv"
	//"strings"
	"sync"
	"time"
)

// Station represents a station server that can relay signals to other stations, and can broadcast signals to the end-clients.
type Station struct {
	Token         string
	Time          time.Time
	Info          *base.ServerInfo
	ChangeHandler func(string, int)

	clientCount       int
	clientCountChange chan int
	broadcasted       map[string]time.Time
	channels          map[string]*Channel
	relays            map[string]*Relay
	recorders         map[string]*Recorder
	relayLocker       sync.Mutex
	isTrunk           bool
}

func (this *Station) InitWith(info *base.ServerInfo, token string) {
	this.Info = info
	this.Token = token
	this.isTrunk = info.Mode&base.STATION_MODE_TRUNK == base.STATION_MODE_TRUNK
	this.channels = make(map[string]*Channel)
	this.relays = make(map[string]*Relay)
	this.recorders = make(map[string]*Recorder)
	this.broadcasted = make(map[string]time.Time)
	this.clientCountChange = make(chan int)
	this.clientCount = 0
	this.Time = time.Now()

	go this.listenClientCountChange()
	go this.reduceBroadcasted()
}

func (this *Station) ClientJoin(ws *websocket.Conn) {
	err, cid, pid := this.parseParams(ws)
	if err != nil {
		websocket.JSON.Send(ws, err)
		return
	}

	channel := this.getChannel(cid)
	this.initClient(ws, pid, channel)
}

func (this *Station) SetRecorders(remoteAddrs []string) {
	if remoteAddrs == nil || len(remoteAddrs) == 0 {
		return
	}
	for _, remoteAddr := range remoteAddrs {
		this.dialRecorder(remoteAddr)
	}
}

func (this *Station) RelayJoin(ws *websocket.Conn) {
	this.relayLocker.Lock()
	flag, relay := this.relayJoin(ws)
	this.relayLocker.Unlock()
	if flag {
		this.startRelay(relay)
	}
}

func (this *Station) RelayWithStation(remoteAddr string) {
	this.relayLocker.Lock()
	flag, relay := this.relayWithStation(remoteAddr)
	this.relayLocker.Unlock()
	if flag {
		this.startRelay(relay)
	}
}

func (this *Station) Broadcast(signal *SignalPack) {
	if this.ExistsChannel(signal.CID) {
		if this.IsBroadcasted(signal.Signal.ID) {
			return
		}
		this.getChannel(signal.CID).Broadcast(signal)
	} else {
		go this.RelayToRemoteStations(signal)
		go this.RecordSignal(signal)
	}
}

func (this *Station) RelayToRemoteStations(signal *SignalPack) {
	if this.IsBroadcasted(signal.Signal.ID) {
		return
	}
	signal.Stations = append(signal.Stations, this.Info.Addr())
	transLen := len(signal.Stations)
	lastAddr := signal.Stations[transLen-1]

	for _, relay := range this.relays {
		if transLen == 1 || relay.Info.Remote.IpAddr != lastAddr && (!this.isTrunk || !relay.RemoteIsTrunk) {
			relay.PushSignal(signal)
		}
	}
}

func (this *Station) RecordSignal(signal *SignalPack) {
	for _, recorder := range this.recorders {
		recorder.PushSignal(signal)
	}
}

func (this *Station) ClientCount() int {
	return this.clientCount
}

func (this *Station) Channels() []*Channel {
	channels := []*Channel{}
	for _, channel := range this.channels {
		channels = append(channels, channel)
	}
	return channels
}

func (this *Station) Relays() []*Relay {
	relays := []*Relay{}
	for _, relay := range this.relays {
		relays = append(relays, relay)
	}
	return relays
}

func (this *Station) Recorders() []*Recorder {
	recorders := []*Recorder{}
	for _, recorder := range this.recorders {
		recorders = append(recorders, recorder)
	}
	return recorders
}

func (this *Station) ChannelCount() int {
	return len(this.channels)
}

func (this *Station) RelayCount() int {
	return len(this.relays)
}

func (this *Station) ChannelClientCount(cid string) int {
	if this.channels[cid] != nil {
		return this.channels[cid].ClientCount()
	}
	return 0
}

func (this *Station) BroadcastedCount() int {
	return len(this.broadcasted)
}

func (this *Station) GetRelayByUPID(upid string) *Relay {
	return this.relays[upid]
}

func (this *Station) ExistsChannel(cid string) bool {
	return this.channels[cid] != nil
}

func (this *Station) IsBroadcasted(id string) bool {
	if !this.broadcasted[id].IsZero() {
		return true
	}
	this.broadcasted[id] = time.Now()
	return false
}

func (this *Station) reduceBroadcasted() {
	time.AfterFunc(base.STATION_BROADCASTED_CACHE_TIMEOUT, func() {
		now := time.Now()
		for id, time := range this.broadcasted {
			if now.Sub(time) >= base.STATION_BROADCASTED_CACHE_TIMEOUT {
				delete(this.broadcasted, id)
			}
		}
		this.reduceBroadcasted()
	})
}

func (this *Station) listenClientCountChange() {
	for {
		select {
		case clientCount := <-this.clientCountChange:
			this.clientCount = this.clientCount + clientCount
		}
	}
}

func (this *Station) getChannel(cid string) *Channel {
	var channel *Channel
	if channel = this.channels[cid]; channel == nil {
		channel = &Channel{}
		channel.InitWith(cid, this)
		channel.BeforeBroadcastHandler = this.beforeBroadcastHandler
		this.channels[cid] = channel
		go channel.Run()
		log.Println("station - channel: opened:", cid)
	}
	return channel
}

func (this *Station) dialRecorder(remoteAddr string) {
	defer time.AfterFunc(base.STATION_TRY_RECONNECT_RECORDER_INTERVAL, func() {
		this.dialRecorder(remoteAddr)
	})
	uri := base.WEBSOCKET_PREFIX + remoteAddr + base.RECORDER_STATION_JOIN_PATH
	ws, err := websocket.Dial(uri, "", "http://localhost/")
	if err != nil {
		log.Println("station - recorder: connect error:", err)
		return
	}
	if ws != nil {
		this.startRecord(ws, remoteAddr)
	}
}

func (this *Station) startRecord(ws *websocket.Conn, remoteAddr string) {
	var recorder *Recorder
	var err error
	if recorder, err = this.createRecorder(ws, remoteAddr); err != nil {
		return
	}
	upid := recorder.Info.UPID
	this.recorders[upid] = recorder
	defer this.releaseRecorder(recorder)
	this.fireParticipantChange(upid, base.ROUTECMDTYPE_RECORDERJOIN)
	log.Println("station - recorder: ready:", upid)
	go recorder.StartListen()
	recorder.StartBroadcast()
}

func (this *Station) createRecorder(ws *websocket.Conn, remoteAddr string) (*Recorder, error) {
	remoteInfo, err := base.SwitchServerInfo(ws, this.Info)
	if err != nil {
		return nil, err
	}
	upid := this.createUPID(ws, remoteInfo.Addr())
	info := ParticipantStruct{
		UPID:    upid,
		PID:     remoteAddr,
		Remote:  &base.RemoteInfo{Conn: ws, IpAddr: remoteAddr},
		Signals: make(chan *SignalPack, 100),
	}

	recorder := &Recorder{
		RemoteSID: remoteInfo.SID,
		Info:      info,
		Station:   this,
		Time:      time.Now(),
	}
	return recorder, nil
}

func (this *Station) releaseRecorder(recorder *Recorder) {
	upid := recorder.Info.UPID
	this.recorders[upid] = nil
	recorder.Release()
	delete(this.recorders, upid)
	recorder = nil
	this.fireParticipantChange(upid, base.ROUTECMDTYPE_RECORDERQUIT)
	log.Println("station - recorder: disconnected:", upid)
}

func (this *Station) relayJoin(ws *websocket.Conn) (bool, *Relay) {
	remoteInfo, err := base.SwitchServerInfo(ws, this.Info)
	if err != nil {
		log.Println("station - relay: join error:", err)
		return false, nil
	}
	remoteAddr := remoteInfo.Addr()
	if exists, relay := this.existsRelay(remoteAddr); exists {
		return false, relay
	}
	upid := this.createUPID(ws, remoteAddr)
	relay := this.createRelay(ws, upid, remoteInfo, remoteAddr)
	return true, relay
}

func (this *Station) relayWithStation(remoteAddr string) (bool, *Relay) {
	if exists, relay := this.existsRelay(remoteAddr); exists {
		this.fireParticipantChange(relay.Info.UPID, base.ROUTECMDTYPE_RELAYEXISTS)
		return false, relay
	}
	uri := base.WEBSOCKET_PREFIX + remoteAddr + base.STATION_RELAY_JOIN_PATH
	ws, err := websocket.Dial(uri, "", "http://localhost/")
	if err != nil {
		log.Println("station - relay: remote station error:", err)
		return false, nil
	}

	var remoteInfo *base.ServerInfo
	if remoteInfo, err = base.SwitchServerInfo(ws, this.Info); err != nil {
		log.Println("station - relay: remote station error:", err)
		return false, nil
	}

	upid := this.createUPID(ws, remoteAddr)
	relay := this.createRelay(ws, upid, remoteInfo, remoteAddr)
	return true, relay
}

func (this *Station) createUPID(ws *websocket.Conn, remoteAddr string) string {
	localAddr := this.Info.Addr()
	var upid string
	if ws.IsClientConn() {
		upid = localAddr + "-" + remoteAddr
	} else {
		upid = remoteAddr + "-" + localAddr
	}
	return upid
}

func (this *Station) createRelay(ws *websocket.Conn, upid string, remoteInfo *base.ServerInfo, remoteAddr string) *Relay {
	info := ParticipantStruct{
		UPID:    upid,
		PID:     remoteAddr,
		Remote:  &base.RemoteInfo{Conn: ws, IpAddr: remoteAddr},
		Signals: make(chan *SignalPack, 100),
	}

	relay := &Relay{
		RemoteSID:     remoteInfo.SID,
		RemoteMode:    remoteInfo.Mode,
		RemoteIsTrunk: remoteInfo.Mode&base.STATION_MODE_TRUNK == base.STATION_MODE_TRUNK,
		Info:          info,
		Station:       this,
		IsRequester:   ws.IsClientConn(),
		Time:          time.Now(),
	}
	this.relays[upid] = relay
	return relay
}

func (this *Station) startRelay(relay *Relay) {
	defer this.releaseRelay(relay)
	this.fireParticipantChange(relay.Info.UPID, base.ROUTECMDTYPE_RELAYJOIN)
	log.Println("station - relay: joined:", relay.Info.UPID)
	go relay.StartListen()
	relay.StartBroadcast()
}

func (this *Station) releaseRelay(relay *Relay) {
	upid := relay.Info.UPID
	this.relays[upid] = nil
	relay.Release()
	delete(this.relays, upid)
	relay = nil
	this.fireParticipantChange(upid, base.ROUTECMDTYPE_RELAYQUIT)
	log.Println("station - relay: quited:", upid)
}

func (this *Station) existsRelay(remoteAddr string) (bool, *Relay) {
	for _, relay := range this.relays {
		if relay.Info.Remote.IpAddr == remoteAddr {
			return true, relay
		}
	}
	return false, nil
}

func (this *Station) initClient(ws *websocket.Conn, pid string, channel *Channel) {
	client := this.clientJoin(ws, pid, channel)
	defer this.clientQuit(client, channel)
	client.StartBroadcast()
}

func (this *Station) clientJoin(ws *websocket.Conn, pid string, channel *Channel) *Client {
	defer func() {
		recover()
	}()
	ipAddr := ws.Request().RemoteAddr
	upid := pid + "_" + ipAddr
	if client := channel.GetClientByUPID(upid); client != nil {
		_ = client.Info.Remote.Conn.Close()
	}

	info := ParticipantStruct{
		UPID:    upid,
		PID:     pid,
		Remote:  &base.RemoteInfo{Conn: ws, IpAddr: ipAddr},
		Signals: make(chan *SignalPack, 100),
	}

	client := &Client{Info: info, Channel: channel}

	channel.ClientJoin(client)

	log.Println("station - client: joined in:", pid)

	go client.StartListen()

	signalPack := SignalPack{
		CID:      channel.CID,
		Time:     time.Now(),
		Stations: []string{},
	}
	signalPack.Signal = Signal{
		ID:   uuid.New(),
		PID:  client.Info.UPID,
		Type: base.SIGNALTYPE_PJOIN,
		Text: strconv.Itoa(len(channel.Clients())),
	}
	channel.Broadcast(&signalPack)
	return client
}

func (this *Station) clientQuit(client *Client, channel *Channel) {
	channel.ClientQuit(client)
	client.Close()

	signalPack := SignalPack{
		CID:      channel.CID,
		Time:     time.Now(),
		Stations: []string{},
	}
	signalPack.Signal = Signal{
		ID:   uuid.New(),
		PID:  client.Info.UPID,
		Type: base.SIGNALTYPE_PQUIT,
		Text: strconv.Itoa(len(channel.Clients())),
	}
	log.Println("station - client: quitted:", client.Info.PID)
	channel.Broadcast(&signalPack)

	if len(channel.Clients()) == 0 {
		this.releaseChannel(channel)
	}
}

func (this *Station) releaseChannel(channel *Channel) {
	time.Sleep(500 * time.Millisecond)
	if len(channel.Clients()) == 0 {
		delete(this.channels, channel.CID)
		channel.Close()
		log.Println("station - channel: closed:", channel.CID)
	}
}

func (this *Station) fireParticipantChange(upid string, cmdType int) {
	go func(upid string, cmdType int) {
		if this.ChangeHandler != nil {
			this.ChangeHandler(upid, cmdType)
		}
	}(upid, cmdType)
}

func (this *Station) parseParams(ws *websocket.Conn) (error, string, string) {
	var channelid string
	var token string
	request := ws.Request()
	request.ParseForm()
	if channelid = request.Form.Get("cid"); channelid == "" {
		return errors.New("no cid"), "", ""
	}

	if token = request.Form.Get("token"); token == "" {
		return errors.New("no token"), "", ""
	}
	return nil, channelid, token
}

func (this *Station) beforeBroadcastHandler(channel *Channel, signal *SignalPack) bool {
	if signal.Signal.Type == base.SIGNALTYPE_PJOIN {
		if len(signal.Stations) == 0 {
			this.clientCountChange <- 1
			this.fireParticipantChange(signal.Signal.PID, base.ROUTECMDTYPE_CLIENTJOIN)
		}
	}
	if signal.Signal.Type == base.SIGNALTYPE_PQUIT {
		if len(signal.Stations) == 0 {
			this.clientCountChange <- -1
			this.fireParticipantChange(signal.Signal.PID, base.ROUTECMDTYPE_CLIENTQUIT)
		}
	}
	return true
}
