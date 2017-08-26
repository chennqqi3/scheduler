package sync
 
import (
 
       // "runtime"
       "fmt"
       "sync"
       "time"
 
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       appStorage "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/common/util/taskpool"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/netservice/client"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm"
)
 
type SyncProvider interface {
       Sync(timeSecond int)
       HealthCheck(timeSecond int)
       CheckStale(timeSecond int)
       RequireResource(timeSecond int)
}

type SyncState struct {
       CreatingState realstate.DockerRunStatusProvider
       Config        ConfigFunc
       algo          *algorithm.Config
       SyncType      map[string]SchedulerProvider
       appContext    *UpdateAppContext
       sem           chan struct{}
       tasks         realstate.HandleTaskDriver
       flowpool      taskpool.TaskPoolInterface
}
 
func NewSyncState(
       creatingState realstate.DockerRunStatusProvider,
       configfunc ConfigFunc,
       algo *algorithm.Config,
	   ) SyncProvider {
       var syncType = map[string]SchedulerProvider{
              appStorage.AppTypeLRC: NewContainerManager(
                     g.RealState,
                     creatingState,
                     g.RealNodeState,
                     configfunc,
                     algo,
                     g.UpdateAppStatusByName,
              ),
			  appStorage.AppTypeSRC: NewContainerOOTManager(
                     g.RealState,
                     creatingState,
                     g.RealNodeState,
                     configfunc,
                     algo,
                     g.UpdateAppStatusByName,
              ),
       }
       if g.Config().NetService.Switch {
              syncType[appStorage.AppTypeLRVM] = NewVMManager(
                     g.RealState,
                     creatingState,
                     g.RealNodeState,
                     configfunc,
                     algo,
                     g.UpdateAppStatusByName,
                     g.UpdateHostnameStatus,
                     g.GetRegionByNode,
              )
       }
	   taskdrv, err := realstate.NewTaskHandler(g.RedisConnPool)
       if nil != err {
              glog.Fatal("Create task handler failed. Error: ", err)
       }
       pool, err := taskpool.NewTaskPool(500)
       if nil != err {
              glog.Fatal("NewTaskPool failed. Error: ", err)
       }
       return &SyncState{
              CreatingState: creatingState,
              Config:        configfunc,
              algo:          algo,
              SyncType:      syncType,
              appContext:    NewUpdateAppContext(),
              sem:           make(chan struct{}, g.Config().MaxDockerExecCount),
              tasks:         taskdrv,
              flowpool:      pool,
       }
}

type InProgress struct {
       InProgressApp map[string]int64
       lock          sync.Mutex
}
 
var appInProgress InProgress
 
func initAppInProgress() {
       appInProgress.InProgressApp = make(map[string]int64)
}
 
const syncExecTimeOut int64 = 1000
 
func (s *SyncState) acquire(appname string) error {
       appInProgress.lock.Lock()
       defer appInProgress.lock.Unlock()
       value, exists := appInProgress.InProgressApp[appname]
       if exists && time.Now().Unix()-value < syncExecTimeOut {
              return fmt.Errorf("appname: %s is in progress", appname)
       }

appInProgress.InProgressApp[appname] = time.Now().Unix()
       s.sem <- struct{}{}
       return nil
}
 
func (s *SyncState) release(appname string) {
       <-s.sem
       appInProgress.lock.Lock()
       delete(appInProgress.InProgressApp, appname)
       appInProgress.lock.Unlock()
}
 
func (s *SyncState) Sync(timeSecond int) {
       initAppInProgress()
       duration := time.Duration(timeSecond) * time.Second
       for {
              time.Sleep(duration)
              // maintenance mode
              if s.Config().MAINT.Switch {
                     s.maintenance()
                     continue
              }
			  // normal mode
              desiredState, err := g.GetDesiredState()
              if err != nil {
                     glog.Error("get desired state fail: ", err)
                     continue
              }
              glog.Infof("Sync Start, there is %d apps need compare.", len(desiredState))
              for _, app := range desiredState {
                     s.syncAppTask(app)
              }
              s.sync(&desiredState)
       }
}
 
func (s *SyncState) syncAppTask(app *appStorage.App) {
       if err := s.acquire(app.Name); err != nil {
              glog.Warningf("acquire channel for desirestate fail: %v", err)
              return
       }

go func(app appStorage.App) {
              defer s.release(app.Name)
              provider, exist := s.SyncType[app.AppType]
              if !exist {
                     glog.Errorf("apptype: '%s' do not match anything", app.AppType)
                     g.UpdateAppStatusByName(app.Name, appStorage.AppStatusSchedulerTypeFail, "app status scheduler type fail")
                     return
              }
              if app.Status == appStorage.AppStatusPending {
                     if err := provider.StatusCompare(&app); nil != err {
                            glog.Errorf("status compared failed. App: %s; Error: %v", app.Name, err)
                     }
              }
              if g.RealState.RealAppExists(app.Name) {
                     err := s.appContext.GetUpdateAppContext(&app).UpdateContainers(provider, &app)
					 if err != nil {
                            glog.Errorf("app %s update fail: %v", app.Name, err)
                     }
                     return
              }
              if app.Status == appStorage.AppStatusPending || app.Status == appStorage.AppStatusCreateSuccess || app.Status == appStorage.AppStatusStartSuccess {
                     if err := provider.CreateContainers(&app); nil != err {
                            glog.Errorf("create containers failed. Error: %v", err)
                     }
              }
       }(*app)
}
 
func (s *SyncState) sync(desiredState *map[string]*appStorage.App) {
 
       realNames := g.RealState.Keys()
       for _, name := range realNames {
              _, exist := (*desiredState)[name]
			  if !exist {
                     ctnrs := g.RealState.Containers(name)
                     if len(ctnrs) < 1 {
                            continue
                     }
                     glog.Warningf("DropContainers, because user delete app, name: %s, num: %d", name, len(ctnrs))
 
                     containers := make([]realstate.Container, 0)
                     for _, ctnr := range ctnrs {
                            containers = append(containers, *ctnr)
                     }
                     ME.NewEventReporter(ME.DeleteContainersBecauseTheAppItBelongsWasDeleted,
                            ME.DeleteContainersBecauseTheAppItBelongsWasDeletedData{
                                   AppName:    name,
                                   Containers: containers,
								   Message:    fmt.Sprintf("App %v was delete by user, delete Containers to --> container count: %v", name, len(containers)),
                            })
 
                     container := ctnrs[0]
                     provider, exist := s.SyncType[container.AppType]
                     if !exist {
                            glog.Errorf("apptype: '%s' do not match anything", container.AppType)
                            g.RealState.DeleteContainer(container.AppName, container)
                            continue
                     }
                     if err := provider.DropContainers(container.AppName, 0, ctnrs); nil != err {
                            glog.Errorf("DropContainers failed. Error: %v", err)
                     }
              }
       }
	   realNamesInCreatingStatus, err := s.CreatingState.GetAllCreatingStatus()
       if err != nil {
              glog.Error("CreatingState.GetAllCreatingStatus fail: ", err)
              return
       }
       for _, appName := range realNamesInCreatingStatus {
              if _, exist := (*desiredState)[appName]; !exist {
                     creatingStatus, err := s.CreatingState.GetCreatingStatus(appName)
                     if err != nil {
                            glog.Error("CreatingState.GetCreatingStatus fail: ", err)
                            continue
                     }
                     if creatingStatus.Count == creatingStatus.Result.Fail+creatingStatus.Result.Success && 0 == len(g.RealState.Containers(appName)) {
                            glog.Infof("DropContainers task finish, name: %v, num: %d", appName, creatingStatus.Count)
                            s.CreatingState.DockerStatusDelete(appName)
                     }
              }
       }
}

func (s *SyncState) maintenance() {
       glog.Infof("################################ MAINT ################################")
       desiredState, err := g.GetDesiredState()
       if err != nil {
              glog.Error("get desired state fail: ", err)
              return
       }
 
       for name, app := range desiredState {
              _, exist := s.SyncType[app.AppType]
              if !exist {
                     glog.Errorf("app type: '%s' do not match anything", app.AppType)
					 g.UpdateAppStatusByName(app.Name, appStorage.AppStatusSchedulerTypeFail, "app status scheduler type fail")
                     continue
              }
 
              if g.RealState.RealAppExists(name) {
                     ctnrs := g.RealState.Containers(name)
                     if len(ctnrs) != app.Instance {
                            glog.Debugf("Need to update app's (name:%s) containers from count %d to %d",
                                   name, len(ctnrs), app.Instance)
                     }
                     for _, c := range ctnrs {
                            // check upgrade status
                            if UpdateCompareFunc(app, c) {
                                   glog.Debugf("app name:%s need to be upgraded", name)
                            }
							// check container health status
                            if realstate.CTNR_HEALTH_ALIVE != c.HealthMode {
                                   glog.Debugf("health check container id:%s failed. appname:%s, health mode:%s.",
                                          c.ID, c.AppName, c.HealthMode)
                            }
                     }
              } else {
                     glog.Debugf("New app, need to create its %s container(s).", name, app.Instance)
              }
       }
 
       realNames := g.RealState.Keys()
       for _, name := range realNames {
              _, exist := desiredState[name]
              if !exist {
                     ctnrs := g.RealState.Containers(name)
                     if len(ctnrs) < 1 {
                            continue
                     }
					 glog.Debugf("app name:%s has been deleted, need to drop its %d containers", name, len(ctnrs))
              }
       }
}
 
func (s *SyncState) CheckStale(timeSecond int) {
       duration := time.Duration(timeSecond) * time.Second
       time.Sleep(duration)
       historyMode := s.Config().MAINT.Switch
       for {
              time.Sleep(duration)
              mode := s.Config().MAINT.Switch
              if (mode != historyMode) && (false == mode) {
                     // scheduler change mode from maint to normal,
                     // wait for 1 minute to start handle stale node and container.
                     glog.Warning("check stale model change from maint mode to normal mode, start sleep for a while for time reason.")
                     time.Sleep(time.Duration(4*timeSecond) * time.Second)
					 glog.Warning("check stale model awake from sleep, and start working.")
              }
 
              if historyMode != mode {
                     historyMode = mode
              }
 
              if mode {
                     // maintenance mode
                     continue
              }
              s.checkStale()
       }
}
 
func (s *SyncState) checkStale() {
       now := time.Now().Unix()
       before := now - 3*int64(s.Config().Interval)
       after := now - 20*int64(s.Config().Interval)
       deleteNodeIPs := g.DeleteStaleNode(before)
       for _, deleteNodeIP := range deleteNodeIPs {
              g.DeleteContainerByIP(deleteNodeIP)
       }
	   deleteContainers := g.RealState.GetStableContainers(before)
       for name, containers := range deleteContainers {
              TimeOutContainers := make([]*realstate.Container, 0)
              for _, container := range containers {
                     getNode, exist := g.RealNodeState.GetNode(container.IP)
                     if !exist {
                            continue
                     }
                     if getNode.HeartBeatAt >= before || getNode.HeartBeatAt <= after {
                            //check if heart beat of node detail do not push, delete containers
                            TimeOutContainers = append(TimeOutContainers, container)
                     } else {
                            glog.Warningf("Minion %v detail heartbeat Timeout, so container %v need update", container.IP, container.ID)
                     }
              }
              deleteContainers[name] = TimeOutContainers
       }
	   g.ProcessStaleContainers(deleteContainers)
}
 
func (s *SyncState) RequireResource(timeSecond int) {
       duration := time.Duration(timeSecond) * time.Second
       for {
              time.Sleep(duration)
              // maintenance mode
              if s.Config().MAINT.Switch {
                     continue
              }
              s.requireResource()
       }
}
 
func (s *SyncState) requireResource() {
       resource, err := g.GetRequireResourceState()
       if err != nil {
              glog.Error("get require resource state fail: ", err)
              return
       }
       for name, app := range resource {
              glog.Infof("%s, require resource Start...", name)
              provider, exist := s.SyncType[app.AppType]
              if !exist {
                     glog.Errorf("apptype: '%s' do not match anything", app.AppType)
					 g.UpdateAppStatusByName(app.Name, appStorage.AppStatusSchedulerTypeFail, "app status scheduler type fail")
                     continue
              }
 
              err := provider.RequireResource(app)
              if err != nil {
                     glog.Errorf("require resource fail: %v", err)
                     ME.NewEventReporter(ME.ExecResult, ME.ExecResultData{
                            Reason:   "require resource failed.",
                            AppName:  app.Name,
                            Error:    err.Error(),
                            Birthday: time.Now(),
                            Message:  fmt.Sprintf("Require Network Resource Fail  --> %v", err),
                     })
                     continue
              }
       }
}

func (s *SyncState) HealthCheck(timeSecond int) {
       go s.networkResourceHealthCheck()
       duration := time.Duration(timeSecond) * time.Second
       for {
              time.Sleep(duration)
              // maintenance mode
              if s.Config().MAINT.Switch {
                     continue
              }
              s.healthCheck()
       }
}
 
func (s *SyncState) healthCheck() {
       resource, err := g.GetHealthCheckState()
       if err != nil {
              glog.Error("health check, get state fail: ", err)
              return
       }
 
       for name, app := range resource {
              if !g.RealState.RealAppExists(name) {
                     glog.Warning("health check app do not exist: ", name)
                     continue
              }
              s.flowpool.Require()
			  go func(app appStorage.App) {
                     defer s.flowpool.Release()
                     provider, exist := s.SyncType[app.AppType]
                     if !exist {
                            glog.Errorf("apptype: '%s' do not match anything", app.AppType)
                            g.UpdateAppStatusByName(app.Name, appStorage.AppStatusSchedulerTypeFail, "app status scheduler type fail")
                            return
                     }
                     if err := provider.HealthCheck(&app); err != nil {
                            glog.Errorf("%s healthCheck fail, name: %s, err: %v", app.AppType, app.Name, err)
                            ME.NewEventReporter(ME.ExecResult, ME.ExecResultData{
                                   Reason:   "health check failed",
                                   AppName:  app.Name,
                                   Error:    err.Error(),
								   Birthday: time.Now(),
                                   Message:  fmt.Sprintf("App %v Health check fail  --> %v", app.Name, err),
                            })
                     }
              }(*app)
       }
}
 
func (s *SyncState) networkResourceHealthCheck() {
       duration := time.Minute * 5
       for {
              time.Sleep(duration)
              if !g.Config().NetService.Switch {
                     continue
              }
              apps, err := g.GetOrphanNetworkApps()
              if err != nil {
                     glog.Error("g.GetOrphanNetworkApps failed:", err)
                     continue
              }
 
              for _, oneApp := range apps {
                     if len(oneApp.Hostnames) <= 0 {
                            glog.Debug("Unexpected result from g.GetOrphanNetworkApps")
                            continue
                     }
					 apply := netservice.Applicants{
                            Hostname:  oneApp.Hostnames[0].Hostname,
                            Subdomain: oneApp.Hostnames[0].Subdomain,
                            Region:    oneApp.Region,
                     }
                     glog.Warningf("releasing network: %s", apply.Hostname+"."+apply.Subdomain)
 
                     _, err := g.NetClient().ReleaseNetwork(apply)
                     if err != nil {
                            glog.Errorf("networkResourceHealthCheck: release network resource fail, hostname: %v, err: %v", oneApp, err)
                     }
              }
       }
}
