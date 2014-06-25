package main

import (
	"errors"
	"github.com/Unknwon/goconfig"
	"os"
	"saassoft.net/signaldistribution/base"
	"strconv"
	"strings"
)

type Config struct {
	ServiceMode  base.ServiceMode
	ServicePort  int
	PublishPort  int
	PublishIP    string
	StationMode  base.StationMode
	RouteServers []string
	Recorders    []string
	ServiceSID   string
	ReadErrors   []error
	ConfigFile   *goconfig.ConfigFile
	Nats         map[string]string
}

func (this *Config) LoadFromFile() []error {
	var err error
	this.ConfigFile, err = goconfig.LoadConfigFile("conf.ini")
	if err != nil {
		this.ReadErrors = append(this.ReadErrors, errors.New("read conf.ini file err:"+err.Error()))
	} else {
		this.read_section_service()
		if this.IsStation() {
			this.read_section_station()
		}
		if this.IsRoute() {
			this.read_section_route()
		}
		if this.IsRecorder() {
			this.read_section_recorder()
		}
	}
	this.setDefault()
	this.ConfigFile = nil
	return this.ReadErrors
}

func (this *Config) IsTrunkNode() bool {
	return this.StationMode&base.STATION_MODE_TRUNK == base.STATION_MODE_TRUNK
}

func (this *Config) IsBranchNode() bool {
	return this.StationMode&base.STATION_MODE_BRANCH == base.STATION_MODE_BRANCH
}

func (this *Config) IsLeafNode() bool {
	return this.StationMode&base.STATION_MODE_LEAF == base.STATION_MODE_LEAF
}

func (this *Config) IsStation() bool {
	return this.ServiceMode&base.SERVICE_MODE_STATION == base.SERVICE_MODE_STATION
}

func (this *Config) IsRoute() bool {
	return this.ServiceMode&base.SERVICE_MODE_ROUTE == base.SERVICE_MODE_ROUTE
}

func (this *Config) IsRecorder() bool {
	return this.ServiceMode&base.SERVICE_MODE_RECORDER == base.SERVICE_MODE_RECORDER
}

func (this *Config) setDefault() {
	if this.ServiceMode <= 0 {
		this.ServiceMode = base.DEFAULT_SERVICE_MODE
	}
	if this.ServicePort <= 0 {
		this.ServicePort = base.DEFAULT_SERVICE_PORT
	}
	if this.PublishPort <= 0 {
		this.PublishPort = this.ServicePort
	}
	if this.StationMode <= 0 {
		this.StationMode = base.DEFAULT_STATION_MODE
	}
	if this.ServiceSID == "" {
		hostName, _ := os.Hostname()
		this.ServiceSID = hostName + ":" + strconv.Itoa(this.ServicePort)
	}
	if this.Recorders == nil {
		this.Recorders = []string{}
	}
	if this.RouteServers == nil {
		this.RouteServers = []string{}
	}
}

func (this *Config) read_section_service() {
	this.read_service_sid()
	this.read_service_mode()
	this.read_service_port()
	this.read_service_publiship()
	this.read_service_publishport()
}

func (this *Config) read_service_mode() {
	value, err := this.ConfigFile.Int("service", "mode")
	if err != nil {
		//this.ReadErrors = append(this.ReadErrors, errors.New("read service mode:"+err.Error()))
	}
	if value > 0 {
		this.ServiceMode = base.ServiceMode(value)
	}
}

func (this *Config) read_service_sid() {
	value, err := this.ConfigFile.GetValue("service", "sid")
	if err != nil {
		//this.ReadErrors = append(this.ReadErrors, errors.New("read service sid:"+err.Error()))
	}
	if value != "" {
		this.ServiceSID = value
	}
}

func (this *Config) read_service_publiship() {
	value, err := this.ConfigFile.GetValue("service", "publiship")
	if err != nil {
		//this.ReadErrors = append(this.ReadErrors, errors.New("read service port:"+err.Error()))
	}
	if value != "" {
		value = strings.TrimSpace(value)
		this.PublishIP = value
	}
}

func (this *Config) read_service_publishport() {
	value, err := this.ConfigFile.Int("service", "publishport")
	if err != nil {
		//this.ReadErrors = append(this.ReadErrors, errors.New("read service port:"+err.Error()))
	}
	if value > 0 {
		this.PublishPort = value
	}
}

func (this *Config) read_service_port() {
	value, err := this.ConfigFile.Int("service", "port")
	if err != nil {
		//this.ReadErrors = append(this.ReadErrors, errors.New("read service port:"+err.Error()))
	}
	if value > 0 {
		this.ServicePort = value
	}
}

func (this *Config) read_section_station() {
	this.read_station_mode()
	this.read_station_routeservers()
	this.read_station_recorders()
}

func (this *Config) read_station_mode() {
	value, err := this.ConfigFile.Int("station", "mode")
	if err != nil {
		//this.ReadErrors = append(this.ReadErrors, errors.New("read station mode:"+err.Error()))
	}
	if value > 0 {
		this.StationMode = base.StationMode(value)
	}
}

func (this *Config) read_station_routeservers() {
	value, err := this.ConfigFile.GetValue("station", "routeservers")
	if err != nil {
		//this.ReadErrors = append(this.ReadErrors, errors.New("read station routeserver:"+err.Error()))
	}
	if value != "" {
		rawRouteServers := strings.Split(value, ";")
		this.RouteServers = []string{}
		rsMap := make(map[string]bool)
		for _, rsAddr := range rawRouteServers {
			addr := strings.TrimSpace(rsAddr)
			if addr == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(addr), base.WEBSOCKET_PREFIX) {
				addr = base.SubString(addr, len(base.WEBSOCKET_PREFIX), len(addr)-len(base.WEBSOCKET_PREFIX))
			}
			if rsMap[addr] {
				continue
			}
			rsMap[addr] = true
			this.RouteServers = append(this.RouteServers, addr)
		}
	}
}

func (this *Config) read_station_recorders() {
	value, err := this.ConfigFile.GetValue("station", "recorders")
	if err != nil {
		//this.ReadErrors = append(this.ReadErrors, errors.New("read station mode:"+err.Error()))
	}
	if value != "" {
		rawStations := strings.Split(value, ";")
		this.Recorders = []string{}
		serverMap := make(map[string]bool)
		for _, stationAddr := range rawStations {
			addr := strings.TrimSpace(stationAddr)
			if addr == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(addr), base.WEBSOCKET_PREFIX) {
				addr = base.SubString(addr, len(base.WEBSOCKET_PREFIX), len(addr)-len(base.WEBSOCKET_PREFIX))
			}
			if serverMap[addr] {
				continue
			}
			serverMap[addr] = true
			this.Recorders = append(this.Recorders, addr)
		}
	}
}

func (this *Config) read_section_route() {
	this.read_route_nats()
}

func (this *Config) read_route_nats() {
	value, err := this.ConfigFile.GetValue("route", "nat")
	if err != nil {
		//this.ReadErrors = append(this.ReadErrors, errors.New("read station mode:"+err.Error()))
	}
	this.Nats = make(map[string]string)
	if value == "" {
		return
	}
	value = strings.TrimSpace(value)
	nats := strings.Split(value, ";")
	for _, nat := range nats {
		if !strings.Contains(nat, "-") {
			continue
		}
		natinfo := strings.Split(nat, "-")
		this.Nats[natinfo[0]] = natinfo[1]
	}
}

func (this *Config) read_section_recorder() {
}
