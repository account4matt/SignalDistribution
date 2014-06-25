// Copyright 2014 liveease.com. All rights reserved.

package main

import (
	"code.google.com/p/go.net/websocket"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"saassoft.net/signaldistribution/base"
	"saassoft.net/signaldistribution/recorder"
	"saassoft.net/signaldistribution/route"
	"saassoft.net/signaldistribution/signal"
	"strconv"
)

var config *Config
var station *signal.Station
var routeClients []*route.RouteClient
var routeServer *route.RouteServer
var recorderServer *recorder.RecorderServer
var stopService chan bool
var serverInfo base.ServerInfo

func StartRuntime() {
	mylogger := base.MyLogger{}
	log.SetOutput(mylogger)
	initConfig()
	initServerInfo()
	startService()
	log.Println("runtime: service started at port: ", strconv.Itoa(config.ServicePort), ",sid:"+config.ServiceSID)

	enabledHTMLService()
	enabledStatisticsService()

	if config.IsRoute() {
		initRouteServer()
		log.Println("runtime: route service is enabled.")
	}
	if config.IsRecorder() {
		initRecorderServer()
		log.Println("runtime: recorder service is enabled.")
	}
	if config.IsStation() {
		initStationServer()
		log.Println("runtime: station service is enabled.")
	}

	waitForStop()
	log.Println("runtime: service stoped.")
}

func StopRuntime() {
	stopService <- true
}

func initConfig() {
	config = &Config{}
	readerrors := config.LoadFromFile()
	if len(readerrors) > 0 {
		for e := range readerrors {
			log.Println("runtime: " + readerrors[e].Error())
		}
	}
}

func initServerInfo() {
	serverInfo.SID = config.ServiceSID
	//serverInfo.IP = config.ServiceIP
	serverInfo.Port = config.ServicePort
	serverInfo.Mode = int(config.ServiceMode)
}

func enabledHTMLService() {
	http.Handle("/", http.FileServer(http.Dir(base.SERVICE_HTML_DIR)))
}

func enabledStatisticsService() {
	http.HandleFunc(base.STATION_STATISTICS_PATH, stationStatistics)
}

func initStationServer() {
	station = &signal.Station{}
	ssi := serverInfo
	ssi.Mode = int(config.StationMode)
	station.InitWith(&ssi, "token")
	station.ChangeHandler = changeHandler
	route.RegisterServerCmdHander()
	routeClients = []*route.RouteClient{}
	for _, routeServerAddr := range config.RouteServers {
		routeClient := &route.RouteClient{Station: station, RouteCmdHander: route.ServerCmdHander, ServerAddr: routeServerAddr}
		routeClients = append(routeClients, routeClient)
		go routeClient.Register()
	}
	if config.Recorders != nil && len(config.Recorders) > 0 {
		go station.SetRecorders(config.Recorders)
	}
	http.Handle(base.STATION_CLIENT_JOIN_PATH, websocket.Handler(station.ClientJoin))
	http.Handle(base.STATION_RELAY_JOIN_PATH, websocket.Handler(station.RelayJoin))
}

func changeHandler(upid string, cmdType int) {
	cmd := &base.RouteCmd{Type: base.RouteCmdType(cmdType), Text: upid}
	for _, routeClient := range routeClients {
		go routeClient.Report(cmd)
	}
}

func initRouteServer() {
	route.Nats = config.Nats
	route.RegisterclientCmdHander()
	routeServer = &route.RouteServer{RouteCmdHander: route.ClientCmdHander}

	routeServer.Run()
	http.Handle(base.ROUTE_REGISTER_PATH, websocket.Handler(routeServer.Register))
	http.Handle(base.ROUTE_REALTIME_PATH, websocket.Handler(routeServer.RealTime))
	http.Handle(base.ROUTE_ROUTE_PATH, websocket.Handler(routeServer.Route))
	http.HandleFunc(base.ROUTE_STATISTICS_PATH, routeStatistics)
}

func initRecorderServer() {
	recorderServer = &recorder.RecorderServer{}
	rsi := serverInfo
	recorderServer.InitWith(&rsi, "token")
	http.Handle(base.RECORDER_STATION_JOIN_PATH, websocket.Handler(recorderServer.StationJoin))
	http.Handle(base.RECORDER_FETCH_PATH, websocket.Handler(recorderServer.Fetch))
}

func startService() {
	go func() {
		port := strconv.Itoa(serverInfo.Port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			panic("runtime: ListenAndServe error:" + err.Error())
		}
	}()
}

func waitForStop() {
	for {
		select {
		case stop := <-stopService:
			if stop == true {
				return
			}
		}
	}
}

func stationStatistics(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "\nChannelCount:"+strconv.Itoa(station.ChannelCount()))
	io.WriteString(w, "\nRelayCount:"+strconv.Itoa(station.RelayCount()))
	io.WriteString(w, "\nBroadcastedCount:"+strconv.Itoa(station.BroadcastedCount()))
	io.WriteString(w, "\n")
	io.WriteString(w, "\nClientCount:"+strconv.Itoa(station.ClientCount()))

	channels := station.Channels()
	for _, channel := range channels {
		io.WriteString(w, "\nChannel:"+channel.CID+" Client Count:"+strconv.Itoa(channel.ClientCount()))
	}
}

func routeStatistics(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, routeServer.StructureString())
}
