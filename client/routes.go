package client

import "github.com/tedsuo/rata"

const (
	//method
	GET = "GET"
	PUT = "PUT"

	//func name
	HealthHandle         = "HealthHandler"
	NodesHandle          = "NodesHandler"
	NodeHandle           = "NodeHandler"
	RealStateHandle      = "RealStateHandler"
	AppHandle            = "AppHandler"
	GlobalTopologyHandle = "GlobalTopologyInfo"
	SetContainerStatus   = "SetContainerStatus"
	GetContainerStatus   = "GetContainerStatus"
	GetDockerLogs        = "GetDockerLogs"
	FindVmPoolResource   = "FindVmPoolResource"
	GetAppEvents         = "GetAppEvents"
	GetAppErrorEvents    = "GetAppErrorEvents"
	ListArchiveEvents    = "ListArchiveEvents"
	GetSchedulerConfig   = "GetSchedulerConfig"
)

var Routes = rata.Routes{
	// routes
	{Path: "/health", Method: GET, Name: HealthHandle},
	{Path: "/nodes", Method: GET, Name: NodesHandle},
	{Path: "/node/:ip", Method: GET, Name: NodeHandle},
	{Path: "/real", Method: GET, Name: RealStateHandle},
	{Path: "/app/:name", Method: GET, Name: AppHandle},
	{Path: "/TopologyHandle/", Method: GET, Name: GlobalTopologyHandle},
	{Path: "/container/fastsetting", Method: PUT, Name: SetContainerStatus},
	{Path: "/container/fastsetting/:jobid", Method: GET, Name: GetContainerStatus},
	{Path: "/container/log", Method: "GET", Name: GetDockerLogs},
	{Path: "/findVmPoolResource", Method: "GET", Name: FindVmPoolResource},
	{Path: "/appevents/:name", Method: "GET", Name: GetAppEvents},
	{Path: "/apperrorevents/:name", Method: "GET", Name: GetAppErrorEvents},
	{Path: "/archiveevents", Method: "GET", Name: ListArchiveEvents},
	{Path: "/schedulerconfig", Method: "GET", Name: GetSchedulerConfig},
}
