// Copyright 2014 liveease.com. All rights reserved.

package route

import (
	"code.google.com/p/go.net/websocket"
	"log"
	"saassoft.net/signaldistribution/base"
	"saassoft.net/signaldistribution/signal"
	"strings"
	"time"
)

type RouteClient struct {
	RouteCmdHander func(*signal.Station, base.RouteCmd)
	Station        *signal.Station
	ServerAddr     string
	cmds           chan *base.RouteCmd
	serverConn     *websocket.Conn
	enabled        bool
}

func (this *RouteClient) Register() {
	this.cmds = make(chan *base.RouteCmd, 100)
	go this.connServer()
}

func (this *RouteClient) Report(cmd *base.RouteCmd) {
	if this.enabled {
		this.cmds <- cmd
	}
}

func (this *RouteClient) connServer() {
	defer time.AfterFunc(base.STATION_TRY_RECONNECT_ROUTESERVER_INTERVAL, this.connServer)
	if this.serverConn != nil {
		this.serverConn.Close()
	}
	ws, err := websocket.Dial(this.serverUri(), "", "http://localhost")
	if err != nil {
		log.Println("station - route client: register error:", err)
		return
	}

	if err := websocket.JSON.Send(ws, &this.Station.Info); err != nil {
		return
	}

	var localAddr string
	if err := websocket.JSON.Receive(ws, &localAddr); err != nil {
		return
	}

	if this.Station.Info.IP == "" {
		ipAddr := base.SubString(localAddr, 0, strings.Index(localAddr, ":"))
		this.Station.Info.IP = ipAddr
	}

	this.serverConn = ws
	log.Println("station - route client: registered to server:", this.ServerAddr)
	this.reportStationInfo()
	go this.enableReport()
	this.standby()
}

func (this *RouteClient) reportStationInfo() {
	relays := this.Station.Relays()
	var relaysString string
	for _, relay := range relays {
		relaysString = relaysString + relay.Info.UPID + ";"
	}
	if relaysString != "" {
		cmd := &base.RouteCmd{Type: base.ROUTECMDTYPE_RELAYS, Text: relaysString}
		this.doReport(cmd)
	}
	recorders := this.Station.Recorders()
	var recordersString string
	for _, recorder := range recorders {
		recordersString = recordersString + recorder.Info.UPID + ";"
	}
	if recordersString != "" {
		cmd := &base.RouteCmd{Type: base.ROUTECMDTYPE_RECORDERS, Text: recordersString}
		this.doReport(cmd)
	}

	channels := this.Station.Channels()
	var clientsString string
	for _, channel := range channels {
		clients := channel.Clients()
		for upid, _ := range clients {
			clientsString = clientsString + upid + ";"
		}
	}
	if clientsString != "" {
		cmd := &base.RouteCmd{Type: base.ROUTECMDTYPE_CLIENTS, Text: clientsString}
		this.doReport(cmd)
	}
}

func (this *RouteClient) enableReport() {
	this.enabled = true
	for cmd := range this.cmds {
		if err := this.doReport(cmd); err != nil {
			return
		}
	}
}

func (this *RouteClient) doReport(cmd *base.RouteCmd) error {
	return websocket.JSON.Send(this.serverConn, cmd)
}

func (this *RouteClient) standby() {
	for {
		var cmd base.RouteCmd
		if err := websocket.JSON.Receive(this.serverConn, &cmd); err != nil {
			log.Println("station - route client: disconnected:", this.serverUri)
			return
		}
		if this.RouteCmdHander != nil {
			this.RouteCmdHander(this.Station, cmd)
		}
	}
}

func (this *RouteClient) serverUri() string {
	if this.ServerAddr == "" {
		return this.ServerAddr
	}
	return base.WEBSOCKET_PREFIX + this.ServerAddr + base.ROUTE_REGISTER_PATH
}
