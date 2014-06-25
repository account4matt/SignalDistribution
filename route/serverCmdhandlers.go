// Copyright 2014 liveease.com. All rights reserved.

package route

import (
	"log"
	"saassoft.net/signaldistribution/base"
	"saassoft.net/signaldistribution/signal"
)

var routeCmdHanders map[base.RouteCmdType]func(*signal.Station, string)

func RegisterServerCmdHander() {
	routeCmdHanders = make(map[base.RouteCmdType]func(*signal.Station, string))
	routeCmdHanders[base.ROUTECMDTYPE_RELAYWITH] = routeCmdHandler_RelayWith
}

func ServerCmdHander(station *signal.Station, cmd base.RouteCmd) {
	if routeCmdHanders[cmd.Type] != nil {
		routeCmdHanders[cmd.Type](station, cmd.Text)
	} else {
		log.Println("station - route client: unknown cmd:", cmd.Type)
	}
}

func routeCmdHandler_RelayWith(station *signal.Station, cmdText string) {
	go station.RelayWithStation(cmdText)
}
