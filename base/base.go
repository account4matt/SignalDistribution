// Copyright 2014 liveease.com. All rights reserved.

// Package base provides const defines, base types and common functions.
package base

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Signal types.
const (
	SIGNALTYPE_BLANK  = iota // blank signal
	SIGNALTYPE_SIGNAL        // text signal
	SIGNALTYPE_PJOIN         // participant join signal
	SIGNALTYPE_PQUIT         // participant quit signal
	SIGNALTYPE_CMD           // command signal
	SIGNALTYPE_ERROR         // error singal
)

// Service Mode. It can be multiplicity.
const (
	SERVICE_MODE_STATION  = 1 << iota // service run as station server
	SERVICE_MODE_ROUTE                // service run as route server
	SERVICE_MODE_RECORDER             // service run as recorder server
)

// Station Mode.
const (
	STATION_MODE_TRUNK  = 1 << iota // station run as trunk node
	STATION_MODE_BRANCH             // station run as branch node
	STATION_MODE_LEAF               // station run as leaf node
)

// Route command types;
const (
	ROUTECMDTYPE_BLANK            = iota // blank command
	ROUTECMDTYPE_ROUTE                   // route command
	ROUTECMDTYPE_ROUTEECHO               // route echo command
	ROUTECMDTYPE_RELAYWITH               // relay with other station command
	ROUTECMDTYPE_RELAYWITHECHO           //
	ROUTECMDTYPE_RELAYS                  // station reports all relays with other stations
	ROUTECMDTYPE_RELAYSECHO              //
	ROUTECMDTYPE_RELAYJOIN               // station reports one relay joined
	ROUTECMDTYPE_RELAYJOINECHO           //
	ROUTECMDTYPE_RELAYEXISTS             // station reports relay to specified station is existed
	ROUTECMDTYPE_RELAYEXISTSECHO         //
	ROUTECMDTYPE_RELAYQUIT               // station reports one relay quited
	ROUTECMDTYPE_RELAYQUITECHO           //
	ROUTECMDTYPE_CLIENTS                 // station reports all connected clients
	ROUTECMDTYPE_CLIENTSECHO             //
	ROUTECMDTYPE_CLIENTJOIN              // station reports one client joined
	ROUTECMDTYPE_CLIENTJOINECHO          //
	ROUTECMDTYPE_CLIENTQUIT              // station reports one client quited
	ROUTECMDTYPE_CLIENTQUITECHO          //
	ROUTECMDTYPE_RECORDERS               // station reports all recorders those connected to
	ROUTECMDTYPE_RECORDERSECHO           //
	ROUTECMDTYPE_RECORDERJOIN            // station reports connected to one recorder
	ROUTECMDTYPE_RECORDERJOINECHO        //
	ROUTECMDTYPE_RECORDERQUIT            // station reports disconnected from one recorder
	ROUTECMDTYPE_RECORDERQUITECHO        //
	ROUTECMDTYPE_REPORTRELAY             //
	ROUTECMDTYPE_REPORTRELAYECHO         //
)

// default value defines.
const (
	DEFAULT_SERVICE_PORT = 25152
	DEFAULT_SERVICE_MODE = SERVICE_MODE_STATION
	DEFAULT_STATION_MODE = STATION_MODE_TRUNK

	ROUTE_SERVER_CHECKRELAYS_INTERVAL          = 5 * time.Second // interval of route server checks relays of stations connected are in expected state.
	STATION_TRY_RECONNECT_ROUTESERVER_INTERVAL = 5 * time.Second // interval of station tries to reconnect route server when it disconnected from route server.
	STATION_TRY_RECONNECT_RECORDER_INTERVAL    = 5 * time.Second // interval of station tries to reconnect recorder when it disconnected from recorder.

	STATION_BROADCASTED_CACHE_TIMEOUT = 30 * time.Second

	WEBSOCKET_PREFIX           = "ws://"                  // websocket schema
	STATION_CLIENT_JOIN_PATH   = "/station/client/join"   // path for client to join to station
	STATION_RELAY_JOIN_PATH    = "/station/relay/join"    // path for other station to relay to station
	RECORDER_FETCH_PATH        = "/recorder/fetch"        // path for fetching history signals from recorder
	RECORDER_STATION_JOIN_PATH = "/recorder/station/join" // path for station to join to recorder
	STATION_STATISTICS_PATH    = "/station/stat"          // path for statistics of station
	ROUTE_REGISTER_PATH        = "/route/register"        // path for station to registering to route.
	ROUTE_STATISTICS_PATH      = "/route/stat"            // path for statistics of route
	ROUTE_ROUTE_PATH           = "/route/route"           // path for client to route
	ROUTE_REALTIME_PATH        = "/route/realtime"        // path for realtime viewer to connect
	SERVICE_HTML_DIR           = "./html/"                // local path of html files
)

// SignalType is type of signal,see constants named start with "SIGNALTYPE_".
type SignalType int

// ServiceMode is mode of service,see constants named start with "SERVICE_MODE_".
type ServiceMode int

// StationMode is mode of station,see constants named start with "STATION_MODE_".
type StationMode int

// RouteCmdType is type of route command,see constants named start with "ROUTECMDTYPE_".
type RouteCmdType int

// RemoteInfo represents a remote connection's informationfo.
type RemoteInfo struct {
	IpAddr string
	Conn   *websocket.Conn
}

// ServerInfo represents a server's information.
type ServerInfo struct {
	SID  string
	IP   string
	Port int
	Mode int
}

// Addr retruns server's ip address.
func (this *ServerInfo) Addr() string {
	return this.IP + ":" + strconv.Itoa(this.Port)
}

// RouteCmd represents a route command.
type RouteCmd struct {
	Type RouteCmdType
	Text string // command text
}

type MyLogger struct {
}

func (this MyLogger) Write(p []byte) (n int, err error) {
	return fmt.Print(string(p))

	//return 0, nil
}

// StringInArray returns true if str in array.
func StringInArray(str string, array []string) bool {
	for _, s := range array {
		if s == str {
			return true
		}
	}
	return false
}

// SubString returns the substring from begin point with length in str.
func SubString(str string, begin int, length int) (substr string) {
	rs := []rune(str)
	lth := len(rs)

	if begin < 0 {
		begin = 0
	}
	if begin >= lth {
		begin = lth
	}
	end := begin + length
	if end > lth {
		end = lth
	}

	return string(rs[begin:end])
}

// SwitchServerInfo sends local server information to remote server,and receives remote server information. returns the remote server information.
func SwitchServerInfo(ws *websocket.Conn, localInfo *ServerInfo) (*ServerInfo, error) {
	if err := websocket.JSON.Send(ws, localInfo); err != nil {
		return nil, err
	}
	var remoteInfo ServerInfo
	if err := websocket.JSON.Receive(ws, &remoteInfo); err != nil {
		return nil, err
	}
	if remoteInfo.IP == "" {
		if ws.IsServerConn() {
			remoteAddr := ws.Request().RemoteAddr
			remoteInfo.IP = SubString(remoteAddr, 0, strings.Index(remoteAddr, ":"))
		} else {
			remoteAddr := ws.RemoteAddr().String()
			if strings.HasPrefix(strings.ToLower(remoteAddr), WEBSOCKET_PREFIX) {
				remoteAddr = SubString(remoteAddr, len(WEBSOCKET_PREFIX), len(remoteAddr))
			}
			remoteInfo.IP = SubString(remoteAddr, 0, strings.Index(remoteAddr, ":"))
		}
	}

	if err := websocket.JSON.Send(ws, remoteInfo); err != nil {
		return nil, err
	}
	var returnLocalInfo ServerInfo
	if err := websocket.JSON.Receive(ws, &returnLocalInfo); err != nil {
		return nil, err
	}
	if localInfo.IP == "" && returnLocalInfo.IP != "" {
		localInfo.IP = returnLocalInfo.IP
	}

	return &remoteInfo, nil
}
