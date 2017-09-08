package http
 
import (
       "github-beta.huawei.com/hipaas/scheduler/client"
       "github.com/tedsuo/rata"
       "net/http"
)
 
// Start handle http request.
func New() http.Handler {
       httpHandler := NewHttpHandler()
 
       actions := rata.Handlers{
              // Tasks
              client.HealthHandle:    route(httpHandler.HealthHandler),
              client.NodesHandle:     route(httpHandler.NodesHandler),
              client.NodeHandle:      route(httpHandler.NodeHandler),
              client.RealStateHandle: route(httpHandler.RealStateHandler),
              client.AppHandle:       route(httpHandler.AppHandler),
              //            client.AppMonitHandle:       route(httpHandler.AppMonit),
              //            client.VMMonitHandle:        route(httpHandler.VMMonit),
              client.GlobalTopologyHandle: route(httpHandler.GlobalTopologyInfo),
              client.SetContainerStatus:   route(httpHandler.SetContainerStatus),
              client.GetContainerStatus:   route(httpHandler.GetContainerStatus),
              client.GetDockerLogs:        route(httpHandler.GetDockerLogs),
              client.FindVmPoolResource:   route(httpHandler.FindVmPoolResource),
              client.GetAppEvents:         route(httpHandler.GetAppEvents),
              client.GetAppErrorEvents:    route(httpHandler.GetAppErrorEvents),
              client.ListArchiveEvents:    route(httpHandler.ListArchiveEvents),
              client.GetSchedulerConfig:   route(httpHandler.GetSchedulerConfig),
       }
 
       handler, err := rata.NewRouter(client.Routes, actions)
       if err != nil {
              panic("unable to create router: " + err.Error())
       }
 
       return handler
}
 
func route(f func(w http.ResponseWriter, r *http.Request)) http.Handler {
       return http.HandlerFunc(f)
}
