package http
 
import (
       "encoding/json"
       fastSet "github-beta.huawei.com/hipaas/common/fastsetting"
       "github-beta.huawei.com/hipaas/common/storage/createstate"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/glog"
       "io/ioutil"
       "net/http"
       "strconv"
       //     "github-beta.huawei.com/hipaas/monitor"
       //     cacheStorage "github-beta.huawei.com/hipaas/monitor/client/storage"
       "github-beta.huawei.com/hipaas/common/appevent"
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       NodePackage "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/scheduler/executor"
       "github-beta.huawei.com/hipaas/scheduler/fastsetting"
       "github-beta.huawei.com/hipaas/scheduler/g"
)
 
const (
       ValidNodes   = "ValidNode"
       InvalidNodes = "InvalidNode"
)
 
type HttpHandler struct {
}
 
func NewHttpHandler() *HttpHandler {
       return &HttpHandler{}
}
 
func (this *HttpHandler) HealthHandler(w http.ResponseWriter, r *http.Request) {
       w.Write([]byte("ok"))
}
 
func (this *HttpHandler) NodesHandler(w http.ResponseWriter, r *http.Request) {
       nodes, err := g.RealNodeState.GetAllNode() //nodeCopy []*Node
       if nil != err {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
 
       var IncompatibleNodes []*NodePackage.Node
       var CompatibleNodes []*NodePackage.Node
       for _, node := range nodes {
              if bCompatible := g.Config().MinionMinVersion.IsVersionIncompatible(node.Version); bCompatible {
                     CompatibleNodes = append(CompatibleNodes, node)
              } else {
                     IncompatibleNodes = append(IncompatibleNodes, node)
              }
       }
 
       allnodes := make(map[string][]*NodePackage.Node)
       allnodes[ValidNodes] = CompatibleNodes
       allnodes[InvalidNodes] = IncompatibleNodes
 
       jsNodes, err := json.Marshal(allnodes)
       if nil != err {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
 
       w.Header().Set("Content-Type", "application/json")
       if _, err := w.Write(jsNodes); nil != err {
              glog.Errorf("write nodes fail:%v, content length:%v", err, len(jsNodes))
              return
       }
}
 
//show all real node
func (this *HttpHandler) NodeHandler(w http.ResponseWriter, r *http.Request) {
       con_ip := r.FormValue(":ip")
 
       node, ok := g.RealNodeState.GetNode(con_ip)
       if !ok {
              http.Error(w, "NO found this node", http.StatusInternalServerError)
              return
       }
 
       js, err := json.Marshal(node)
       if err != nil {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
 
       w.Header().Set("Content-Type", "application/json")
       w.Write(js)
}
 
//show all real app
func (this *HttpHandler) RealStateHandler(w http.ResponseWriter, r *http.Request) {
       var containers []*realstate.Container
       for _, key := range g.RealState.Keys() {
              cs := g.RealState.Containers(key)
              for _, container := range cs {
                     containers = append(containers, container)
              }
       }
       js, err := json.Marshal(containers)
       if err != nil {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
       w.Header().Set("Content-Type", "application/json")
       w.Write(js)
}
 
//show real app by name
func (this *HttpHandler) AppHandler(w http.ResponseWriter, r *http.Request) {
       appName := r.FormValue(":name")
       if appName == "" {
              http.Error(w, "name isn't exist", http.StatusInternalServerError)
              return
       }
 
       cs := g.RealState.Containers(appName)
       vs := make([]*realstate.Container, len(cs))
       idx := 0
       for _, v := range cs {
              vs[idx] = v
              idx++
       }
 
       js, err := json.Marshal(vs)
       if err != nil {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
 
       w.Header().Set("Content-Type", "application/json")
       w.Write(js)
}
 
//show real app by name
//func (this *HttpHandler) AppMonit(w http.ResponseWriter, r *http.Request) {
//     var monitInfo []*monitor.ContainerMetrix
//     appName := r.URL.Path[len("/app/monit/"):]
 
//     if appName == "" {
//            http.NotFound(w, r)
//            return
//     }
 
//     cs := g.RealState.Containers(appName)
//     if len(cs) == 0 {
//            http.NotFound(w, r)
//            return
//     }
 
//     redisStorage, err := cacheStorage.NewMonitorStorageDriver(g.RedisConnPool, 0)
//     if err != nil {
//            http.Error(w, err.Error(), http.StatusInternalServerError)
//            return
//     }
 
//     for _, v := range cs {
//            info, err := redisStorage.ContainerStats(v.IP, v.ID, 1)
//            if err != nil {
//                   http.Error(w, err.Error(), http.StatusInternalServerError)
//                   return
//            }
//            for _, monit := range info {
//                   monitInfo = append(monitInfo, monit)
//            }
//     }
 
//     js, err := json.Marshal(monitInfo)
//     if err != nil {
//            http.Error(w, err.Error(), http.StatusInternalServerError)
//            return
//     }
//     w.Header().Set("Content-Type", "application/json")
//     w.Write(js)
//}
 
//func (this *HttpHandler) VMMonit(w http.ResponseWriter, r *http.Request) {
//     machineName := r.URL.Path[len("/vm/monit/"):]
//     if machineName == "" {
//            http.NotFound(w, r)
//            return
//     }
 
//     redisStorage, err := cacheStorage.NewMonitorStorageDriver(g.RedisConnPool, 0)
//     if err != nil {
//            http.Error(w, err.Error(), http.StatusInternalServerError)
//            return
//     }
 
//     machineInfo, err := redisStorage.MachineStats(machineName, 1)
//     if err != nil {
//            http.Error(w, err.Error(), http.StatusInternalServerError)
//            return
//     }
 
//     js, err := json.Marshal(machineInfo)
//     if err != nil {
//            http.Error(w, err.Error(), http.StatusInternalServerError)
//            return
//     }
//     w.Header().Set("Content-Type", "application/json")
//     w.Write(js)
//}
 
func (this *HttpHandler) GlobalTopologyInfo(w http.ResponseWriter, r *http.Request) {
       topo := g.Topology{}
       global, err := topo.GetGlobalTopology()
       if nil != err {
              glog.Error("[ERROR] get topology fail:", err)
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
       js, err := json.Marshal(*global)
       if err != nil {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
       w.Header().Set("Content-Type", "application/json")
       w.Write(js)
}
 
func (this *HttpHandler) SetContainerStatus(w http.ResponseWriter, r *http.Request) {
       body, err := ioutil.ReadAll(r.Body)
       if err != nil {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
 
       var fastSetting *fastSet.FastSetting
       if err = json.Unmarshal (body, & fastSetting); err! = nil {
              http.Error (w, err.Error (), http.StatusInternalServerError)
              return
       }
 
       glog.V (1) .Infof ("[Note] Received FastSetting Request:% v", Fixing)
       determinationInstance, err: = determination.GetFrame ()
       if err! = nil {
              http.Error (w, err.Error (), http.StatusInternalServerError)
              return
       }
       odd, err: = fixInstance.AddFastSettingJob (fixedSetting)
       if err! = nil {
              http.Error (w, err.Error (), http.StatusInternalServerError)
              return
       }
 
       glog.V (1) .Infof ("[Note] Add FastSetting Job Success:% s", uuid)
       js, err: = json.Marshal (& createstate.SettingResponse {JobID: uuid})
       if err! = nil {
              http.Error (w, err.Error (), http.StatusInternalServerError)
              return
       }
       w.Header (). Set ( "Content-Type", "application/json")
       w.Write(js)
}
 
func (this *HttpHandler) GetContainerStatus(w http.ResponseWriter, r *http.Request) {
       jobID := r.FormValue(":jobid")
       if jobID == "" {
              http.Error(w, "job id isn't exist", http.StatusInternalServerError)
              return
       }
       fastsettingInstance, err := fastsetting.GetFastsetting()
       if err != nil {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
       fastSetting, err := fastsettingInstance.GetFastSettingResult(jobID)
       if err != nil {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
       w.Header().Set("Content-Type", "application/json")
       w.Write(fastSetting)
}
 
func (this *HttpHandler) GetDockerLogs(w http.ResponseWriter, req *http.Request) {
       appName := req.FormValue("name")
       if appName == "" {
              http.Error(w, "app name isn't exist", http.StatusInternalServerError)
              return
       }
       ctnrID := req.FormValue("id")
       totalLogs, err := executor.GetDockerManager().GetDockerLogs(appName, ctnrID)
       if err != nil {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
 
       js, err := json.Marshal(&totalLogs)
       if err != nil {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
       w.Header().Set("Content-Type", "application/json")
       w.Write(js)
}
 
func (this *HttpHandler) FindVmPoolResource(w http.ResponseWriter, req *http.Request) {
       vmPoolResources, err := g.FindVmPoolResource()
       if err != nil {
              glog.Error("g.FindVmPoolResource fail: ", err)
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
       // glog.Debug("the size of vmPoolResources is:", len(vmPoolResources))
       js, err := json.Marshal(vmPoolResources)
       if err != nil {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
       w.Header().Set("Content-Type", "application/json")
       w.Write(js)
}
 
func (this *HttpHandler) GetAppEvents(w http.ResponseWriter, req *http.Request) {
       appName := req.FormValue(":name")
       if appName == "" {
              http.Error(w, "name is null", http.StatusInternalServerError)
              return
       }
 
       var err error
       var size int64
       if req.FormValue("size") == "" {
              size = 10
       } else {
              size, err = strconv.ParseInt(req.FormValue("size"), 10, 16)
              if err != nil {
                     glog.Infof("size is invalid:%v", err.Error())
                     http.Error(w, "size is invalid", http.StatusInternalServerError)
                     return
              }
              if size < 0 {
                     glog.Errorf("size can not less than 0:%v", size)
                     http.Error(w, "size can not less than 0", http.StatusInternalServerError)
                     return
              }
              if size > appevent.MaxEventSize {
                     size = appevent.MaxEventSize
              }
       }
       glog.Infof("GetAppEvents:size=%d", size)
       events, err := appevent.ListAppEvents(appName, size)
       if nil != err {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
       eventjson, errOccur := json.Marshal(*events)
       if errOccur != nil {
              http.Error(w, errOccur.Error(), http.StatusInternalServerError)
              return
       }
 
       w.Header().Set("Content-Type", "application/json")
       w.Write(eventjson)
}
 
func (this *HttpHandler) GetAppErrorEvents(w http.ResponseWriter, req *http.Request) {
       appName := req.FormValue(":name")
       if appName == "" {
              http.Error(w, "name is null", http.StatusInternalServerError)
              return
       }
 
       events, err := appevent.ListAppEvents(appName, appevent.LastErrorEvents)
       if nil != err {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
 
       var errevents []appevent.EventInfo
       for i := len(*events) - 1; i >= 0; i-- {
              if (*events)[i].Error != "" {
                     errevents = append(errevents, (*events)[i])
                     if len(errevents) == 3 {
                            break
                     }
              }
       }
       glog.Infof("GetAppErrorEvents app:%s, return %d error events.", appName, len(errevents))
       eventjson, errOccur := json.Marshal(errevents)
       if errOccur != nil {
              http.Error(w, errOccur.Error(), http.StatusInternalServerError)
              return
       }
 
       w.Header().Set("Content-Type", "application/json")
       w.Write(eventjson)
}
 
func (this *HttpHandler) ListArchiveEvents(w http.ResponseWriter, req *http.Request) {
       events, err := ME.ListArchiveEvents()
       if nil != err {
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
       eventsjson, errOccur := json.Marshal(*events)
       if nil != errOccur {
              http.Error(w, errOccur.Error(), http.StatusInternalServerError)
              return
       }
       w.Header().Set("Content-Type", "application/json")
       w.Write(eventsjson)
}
 
func (this *HttpHandler) GetSchedulerConfig(w http.ResponseWriter, req *http.Request) {
       jsconfig, err := json.Marshal(g.Config())
       if err != nil {
              glog.Errorf("GetSchedulerConfig failed:%v", err)
              http.Error(w, err.Error(), http.StatusInternalServerError)
              return
       }
       w.Header().Set("Content-Type", "application/json")
       w.Write(jsconfig)
}
