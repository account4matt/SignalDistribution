// Copyright 2014 liveease.com. All rights reserved.

package route

import (
	"log"
	"saassoft.net/signaldistribution/base"
	"strings"
	"time"
)

var clientCmdHanders map[base.RouteCmdType]func(*RouteServer, *Station, string)

func RegisterclientCmdHander() {
	clientCmdHanders = make(map[base.RouteCmdType]func(*RouteServer, *Station, string))
	clientCmdHanders[base.ROUTECMDTYPE_RELAYS] = clientCmdHandler_Relays
	clientCmdHanders[base.ROUTECMDTYPE_RELAYJOIN] = clientCmdHandler_RelayJoin
	clientCmdHanders[base.ROUTECMDTYPE_RELAYQUIT] = clientCmdHandler_RelayQuit
	clientCmdHanders[base.ROUTECMDTYPE_RELAYEXISTS] = clientCmdHandler_RelayJoin
	clientCmdHanders[base.ROUTECMDTYPE_CLIENTS] = clientCmdHandler_Clients
	clientCmdHanders[base.ROUTECMDTYPE_CLIENTJOIN] = clientCmdHandler_ClientJoin
	clientCmdHanders[base.ROUTECMDTYPE_CLIENTQUIT] = clientCmdHandler_ClientQuit
	clientCmdHanders[base.ROUTECMDTYPE_RECORDERS] = clientCmdHandler_Recorders
	clientCmdHanders[base.ROUTECMDTYPE_RECORDERJOIN] = clientCmdHandler_RecorderJoin
	clientCmdHanders[base.ROUTECMDTYPE_RECORDERQUIT] = clientCmdHandler_RecorderQuit
}

func ClientCmdHander(routeServer *RouteServer, from *Station, cmd base.RouteCmd) {
	if clientCmdHanders[cmd.Type] != nil {
		clientCmdHanders[cmd.Type](routeServer, from, cmd.Text)
	} else {
		log.Println("route server: unknown cmd:", cmd.Type)
	}
}

func clientCmdHandler_Relays(routeServer *RouteServer, from *Station, cmdText string) {
	relays := strings.Split(cmdText, ";")
	for _, relay := range relays {
		clientCmdHandler_RelayJoin(routeServer, from, relay)
	}
}

func clientCmdHandler_RelayJoin(routeServer *RouteServer, from *Station, cmdText string) {
	sids := strings.Split(cmdText, "-")
	if len(sids) != 2 || routeServer.Stations[sids[0]] == nil || routeServer.Stations[sids[1]] == nil {
		return
	}
	isTrunk := routeServer.Stations[sids[0]].Mode&base.STATION_MODE_TRUNK == base.STATION_MODE_TRUNK
	isTrunk = isTrunk && routeServer.Stations[sids[1]].Mode&base.STATION_MODE_TRUNK == base.STATION_MODE_TRUNK
	relay := &Relay{FromSID: sids[0], ToSID: sids[1], Time: time.Now(), IsTrunk: isTrunk}
	from.AppendRelay(cmdText, relay)
	routeServer.structureChange()
}

func clientCmdHandler_RelayQuit(routeServer *RouteServer, from *Station, cmdText string) {
	sids := strings.Split(cmdText, "-")
	if len(sids) != 2 {
		return
	}
	for _, s := range routeServer.Stations {
		if s.RemoteInfo.IpAddr == sids[0] || s.RemoteInfo.IpAddr == sids[1] {
			s.RemoveRelay(cmdText)
			routeServer.structureChange()
		}
	}
}

func clientCmdHandler_RelayExists(routeServer *RouteServer, from *Station, cmdText string) {
	if from.ExistsTrunkRelay(cmdText) || from.ExistsRelay(cmdText) {
		return
	}
	clientCmdHandler_RelayJoin(routeServer, from, cmdText)
}

func clientCmdHandler_Clients(routeServer *RouteServer, from *Station, cmdText string) {
	clients := strings.Split(cmdText, ";")
	for _, client := range clients {
		clientCmdHandler_ClientJoin(routeServer, from, client)
	}
}

func clientCmdHandler_ClientJoin(routeServer *RouteServer, from *Station, cmdText string) {
	from.AppendClient(cmdText)
	routeServer.structureChange()
}

func clientCmdHandler_ClientQuit(routeServer *RouteServer, from *Station, cmdText string) {
	from.RemoveClient(cmdText)
	routeServer.structureChange()
}

func clientCmdHandler_Recorders(routeServer *RouteServer, from *Station, cmdText string) {
	recorders := strings.Split(cmdText, ";")
	for _, recorder := range recorders {
		clientCmdHandler_RecorderJoin(routeServer, from, recorder)
	}
}

func clientCmdHandler_RecorderJoin(routeServer *RouteServer, from *Station, cmdText string) {
	from.AppendRecorder(cmdText)
	routeServer.structureChange()
}

func clientCmdHandler_RecorderQuit(routeServer *RouteServer, from *Station, cmdText string) {
	from.RemoveRecorder(cmdText)
	routeServer.structureChange()
}
