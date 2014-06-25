// Copyright 2014 liveease.com. All rights reserved.

package route

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"log"
	"saassoft.net/signaldistribution/base"
	"strconv"
	"strings"
	"time"
)

// RouteServer is a route server that organizes a cluster of stations, its jobs:
//
// 1. plans the relationship of the stations;
// 2. checks the availability of the stations;
// 3. routes the end-client to an available station and an available recorder server;
type RouteServer struct {
	Stations        map[string]*Station
	Time            time.Time
	RouteCmdHander  func(*RouteServer, *Station, base.RouteCmd)
	changeChan      chan bool
	realTimeReaders map[string]*websocket.Conn
}

// Run starts to service
func (this *RouteServer) Run() {
	this.Stations = make(map[string]*Station)
	this.realTimeReaders = make(map[string]*websocket.Conn)
	this.Time = time.Now()
	this.changeChan = make(chan bool)
	go this.handleChange()
	go this.checkRelays()
}

// Register accepts the station to register to the route server.
func (this *RouteServer) Register(ws *websocket.Conn) {
	station, err := this.initStation(ws)
	if err != nil {
		log.Println("route server - station: register:", err)
		return
	}
	defer this.releaseStation(station)
	log.Println("route server - station: joined:", station.SID)
	this.structureChange()
	this.planRelay(station)
	this.listenStation(station)
	this.structureChange()
	log.Println("route server - station: quited:", station.SID)
	station = nil
}

// Route accepts the end-client to request route.
func (this *RouteServer) Route(ws *websocket.Conn) {
	var pickedStation *Station
	var pickedRecorder string
	for _, st := range this.Stations {
		if pickedRecorder == "" && len(st.Recorders) > 0 {
			for _, recorder := range st.Recorders {
				pickedRecorder = recorder.PublishAddr
				break
			}
		}
		if pickedStation == nil {
			pickedStation = st
			continue
		}

		if len(pickedStation.Clients) > len(st.Clients) {
			pickedStation = st
			if len(st.Recorders) > 0 {
				for _, recorder := range st.Recorders {
					pickedRecorder = recorder.PublishAddr
					break
				}
			}
		}
	}
	if pickedStation == nil {
		websocket.Message.Send(ws, "error:no station")
		return
	}
	websocket.Message.Send(ws, "data:"+pickedStation.PublishAddr+";"+pickedRecorder)
}

// RealTime accepts the observer to fetch the realtime status of the stations cluster.
func (this *RouteServer) RealTime(ws *websocket.Conn) {
	rtId := ws.Request().RemoteAddr
	log.Println("route server - real time reader: joined:", rtId)
	this.realTimeReaders[rtId] = ws
	defer func(rtId string) {
		this.realTimeReaders[rtId] = nil
		delete(this.realTimeReaders, rtId)
		log.Println("route server - real time reader: quited:", rtId)
	}(rtId)
	websocket.Message.Send(ws, this.StructureString())
	this.waitForQuery(ws)
}

// Structure returns the structure of the cluster .
func (this *RouteServer) Structure() []*Station {
	var stations []*Station
	for _, s := range this.Stations {
		stations = append(stations, s)
	}
	return stations
}

// StructureString returns the structure of the cluster.
func (this *RouteServer) StructureString() string {
	var stations []*Station
	var jsonString string
	for _, s := range this.Stations {
		stations = append(stations, s)
	}
	if jsonBytes, err := json.Marshal(stations); err == nil {
		jsonString = string(jsonBytes)
	}

	return jsonString
}

func (this *RouteServer) initStation(ws *websocket.Conn) (*Station, error) {

	var info base.ServerInfo
	if err := websocket.JSON.Receive(ws, &info); err != nil {
		ws.Close()
		return nil, err
	}

	remoteAddr := ws.Request().RemoteAddr
	if err := websocket.JSON.Send(ws, &remoteAddr); err != nil {
		ws.Close()
		return nil, err
	}

	var ipAddr string
	if info.IP != "" {
		ipAddr = info.Addr()
	} else {
		ipAddr = base.SubString(remoteAddr, 0, strings.Index(remoteAddr, ":")+1) + strconv.Itoa(info.Port)
	}
	publishAddr := Nats[ipAddr]
	if publishAddr == "" {
		publishAddr = ipAddr
	}
	station := &Station{
		SID:         info.SID,
		IsOnline:    true,
		Mode:        base.StationMode(info.Mode),
		PublishAddr: publishAddr,
		RemoteInfo: base.RemoteInfo{
			Conn:   ws,
			IpAddr: ipAddr,
		},
		TrunkRelays: make(map[string]*Relay),
		Relays:      make(map[string]*Relay),
		Clients:     make(map[string]time.Time),
		Recorders:   make(map[string]*Recorder),
		Time:        time.Now(),
	}
	this.Stations[ipAddr] = station
	return station, nil
}

func (this *RouteServer) handleChange() {
	for b := range this.changeChan {
		if !b {
			return
		}
		for _, rtr := range this.realTimeReaders {
			if err := websocket.Message.Send(rtr, this.StructureString()); err != nil {
			}
		}
	}
}

func (this *RouteServer) checkRelays() {
	defer this.checkRelays()
	timer := time.NewTimer(base.ROUTE_SERVER_CHECKRELAYS_INTERVAL)
	<-timer.C
	timer = nil
	tss, _, _ := this.statonsClassify()
	this.checkTrunkRelays(tss)
}

func (this *RouteServer) checkTrunkRelays(tss []string) bool {
	tscount := len(tss)
	trcount := tscount - 1
	for _, tsAddr := range tss {
		ts := this.Stations[tsAddr]
		if ts == nil {
			return false
		}
		if len(ts.TrunkRelays) >= trcount {
			continue
		}
		for _, rtsAddr := range tss {
			if tsAddr == rtsAddr {
				continue
			}
			if ts.TrunkRelays[tsAddr+"-"+rtsAddr] != nil || ts.TrunkRelays[rtsAddr+"-"+tsAddr] != nil {
				continue
			}
			this.orderStationRelayWith(ts, rtsAddr)
		}
	}
	return true
}

func (this *RouteServer) statonsClassify() ([]string, []string, []string) {
	tss := []string{}
	bss := []string{}
	lss := []string{}

	for _, s := range this.Stations {
		if s.Mode&base.STATION_MODE_TRUNK == base.STATION_MODE_TRUNK {
			tss = append(tss, s.RemoteInfo.IpAddr)
			continue
		}
		if s.Mode&base.STATION_MODE_BRANCH == base.STATION_MODE_BRANCH {
			bss = append(bss, s.RemoteInfo.IpAddr)
			continue
		}
		if s.Mode&base.STATION_MODE_LEAF == base.STATION_MODE_LEAF {
			lss = append(lss, s.RemoteInfo.IpAddr)
			continue
		}
	}
	return tss, bss, lss
}

func (this *RouteServer) waitForQuery(ws *websocket.Conn) {
	for {
		var cmd string
		if err := websocket.JSON.Receive(ws, &cmd); err != nil {
			return
		}
	}
}

func (this *RouteServer) structureChange() {
	go func() {
		this.changeChan <- true
	}()
}

func (this *RouteServer) planRelay(station *Station) {
	if station.Mode&base.STATION_MODE_TRUNK == base.STATION_MODE_TRUNK {
		this.planTrunkRelay(station)
		return
	}
	if station.Mode&base.STATION_MODE_BRANCH == base.STATION_MODE_BRANCH {
		this.planBranchRelay(station)
		return
	}
}

func (this *RouteServer) planTrunkRelay(station *Station) {
	var tss []*Station = []*Station{}
	for _, s := range this.Stations {
		if s.Mode == base.STATION_MODE_TRUNK && s.RemoteInfo.IpAddr != station.RemoteInfo.IpAddr {
			tss = append(tss, s)
		}
	}
	c := 0
	for i := len(tss) - 1; i >= 0; i-- {
		if c++; c%2 == 0 {
			this.orderStationRelayWith(station, tss[i].RemoteInfo.IpAddr)
		} else {
			this.orderStationRelayWith(tss[i], station.RemoteInfo.IpAddr)
		}
	}
}

func (this *RouteServer) planBranchRelay(station *Station) {
	var pickedStation *Station
	for _, st := range this.Stations {
		if st.Mode == base.STATION_MODE_LEAF || st.RemoteInfo.IpAddr == station.RemoteInfo.IpAddr {
			continue
		}
		if pickedStation == nil {
			pickedStation = st
			continue
		}
		if len(pickedStation.Relays) > len(st.Relays) {
			pickedStation = st
		}
	}
	if pickedStation == nil {
		return
	}
	this.orderStationRelayWith(station, pickedStation.RemoteInfo.IpAddr)
}

func (this *RouteServer) orderStationRelayWith(station *Station, ipAddr string) {
	cmd := &base.RouteCmd{
		Type: base.ROUTECMDTYPE_RELAYWITH,
		Text: ipAddr,
	}
	go this.orderStation(station, cmd)
}

func (this *RouteServer) orderStation(station *Station, cmd *base.RouteCmd) {
	if err := websocket.JSON.Send(station.RemoteInfo.Conn, cmd); err != nil {
	}
}

func (this *RouteServer) listenStation(station *Station) {
	for {
		var cmd base.RouteCmd
		if err := websocket.JSON.Receive(station.RemoteInfo.Conn, &cmd); err != nil {
			return
		}
		if cmd.Type != base.ROUTECMDTYPE_BLANK && this.RouteCmdHander != nil {
			this.RouteCmdHander(this, station, cmd)
		}
	}
}

func (this *RouteServer) releaseStation(station *Station) {
	this.Stations[station.RemoteInfo.IpAddr] = nil
	delete(this.Stations, station.RemoteInfo.IpAddr)
}
