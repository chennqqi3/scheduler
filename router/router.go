package router
 
import (
       "encoding/json"
       "errors"
       "fmt"
       "time"
 
       FS "github-beta.huawei.com/hipaas/common/fastsetting"
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       "github-beta.huawei.com/hipaas/common/routes"
       "github-beta.huawei.com/hipaas/common/routes/routedrivers"
       storageapp "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       TP "github-beta.huawei.com/hipaas/common/util/taskpool"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/scheduler/g"
)
 
const (
       chanCap  int    = 200
       rsPrefix string = "hipaas_rs_"
)
 
var (
       routerInstance Route
)

func StartRouterDrv() error {
       cfg := &routedrivers.ConsulConfig{
              ConsulAddress:    g.Config().ConsulServer.ConsulAddress,
              ConsulScheme:     g.Config().ConsulServer.ConsulScheme,
              ConsulDatacenter: g.Config().ConsulServer.ConsulDatacenter,
              ConsulRoutePath:  g.Config().ConsulServer.ConsulRoutePath,
       }
       drv, err := routedrivers.NewRouteMainDriver(g.Config().Route.PushInterval,
              cfg,
              rsPrefix,
              g.RedisConnPool,
              g.RealState)
       if nil != err {
              return errors.New("start route driver error: "   err.Error())
       }
	   drv.UpdateRouteBackendDrivers(g.Config().Route.BackendRouteDrivers)
 
       pool, poolerr := TP.NewTaskPool(200)
       if nil != poolerr {
              return fmt.Errorf("create task pool failed. Error: %v", poolerr)
       }
 
       routerInstance = Route{
              task:                   pool,
              routerDrv:              drv,
              appStartedChan:         make(chan []byte, chanCap),
              appDeletedChan:         make(chan []byte, chanCap),
              failoverChan:           make(chan []byte, chanCap),
              fastSettingSuccessChan: make(chan []byte, chanCap),
       }
	   if err = routerInstance.registerEventsToME(); err != nil {
              return errors.New("router register to events center error: "   err.Error())
       }
       go routerInstance.waitEvents()
       go routerInstance.routerDrv.TimePushTotalData()
       return nil
}
 
type Route struct {
       task                   TP.TaskPoolInterface
       routerDrv              *routedrivers.RouteMainDriver
       appStartedChan         chan []byte
       appDeletedChan         chan []byte
       failoverChan           chan []byte
       fastSettingSuccessChan chan []byte
}

func (rt *Route) waitEvents() {
       for {
              select {
              case dat := <-rt.appStartedChan:
                     val := ME.AppStatusToStartedData{}
                     if err := json.Unmarshal(dat, &val); nil != err {
                            glog.Error(err)
                            continue
                     }
                     if val.App.AppType == storageapp.AppTypeLRVM {
                            continue
                     }
                     if val.App.Keeproute == storageapp.NO {
                            continue
                     }
 
                     if len(val.App.Health) == 0 {
                            continue
                     }
					 ctnrRoutes := []*routes.ContainerRoute{}
                     for _, container := range val.Containers {
                            ctnrRoute := routes.GenerateRouteData(&container)
                            if ctnrRoute != nil {
                                   ctnrRoutes = append(ctnrRoutes, ctnrRoute)
                            }
                     }
                     rt.task.Execute(func() {
                            if err := rt.routerDrv.AddRoutes(val.App.Name, ctnrRoutes); err != nil {
                                   glog.Errorf("Add router for app %s error: %s", val.App.Name, err.Error())
                            }
                     })
					 case dat := <-rt.appDeletedChan:
                     val := ME.DeleteContainersBecauseTheAppItBelongsWasDeletedData{}
                     if err := json.Unmarshal(dat, &val); nil != err {
                            glog.Error(err)
                            continue
                     }
                     rt.task.Execute(func() {
                            if err := rt.routerDrv.DelRoutes(val.AppName); err != nil {
                                   glog.Errorf("Delete routes for app %s error: %s", val.AppName, err.Error())
                            }
                     })
 
              case dat := <-rt.failoverChan:
                     val := ME.FailoverData{}
                     if err := json.Unmarshal(dat, &val); nil != err {
                            glog.Error(err)
                            continue
                     }
					 val.Container.Status = realstate.ContainerStatusDown
                     for i := 0; i < len(val.Container.PortInfos); i   {
                            if val.Container.PortInfos[i].Portname == realstate.PublicPortName {
                                   val.Container.PortInfos[i].Ports = nil
                                   break
                            }
                     }
                     ctnrRoutes := []*routes.ContainerRoute{}
                     containers := g.RealState.Containers(val.Container.AppName)
                     containers = append(containers, &val.Container)
					 for _, container := range containers {
                            ctnrRoute := routes.GenerateRouteData(container)
                            if ctnrRoute != nil {
                                   ctnrRoutes = append(ctnrRoutes, ctnrRoute)
                                   if ctnrRoute.Port == -1 {
                                          glog.Warningf("route port is -1 for container id: %s", container.ID)
                                   }
                            }
                     }
                     rt.task.Execute(func() {
                            if err := rt.routerDrv.UpdateRoutes(val.Container.AppName, ctnrRoutes); nil != err {
                                   glog.Errorf("UpdateRoutes failed. Error: %v", err)
                            }
                     })
					 case dat := <-rt.fastSettingSuccessChan:
                     val := ME.FastSettingSuccessData{}
                     if err := json.Unmarshal(dat, &val); nil != err {
                            glog.Error(err)
                            continue
                     }
                     app, err := g.GetAppByName(val.AppName)
                     if err != nil {
                            glog.Errorf("update routes after fast setting, GetAppByName error: %s", err.Error())
                            continue
                     }
                     if app.AppType == storageapp.AppTypeLRVM {
                            continue
                     }
					 rt.task.Execute(func() {
                            update := func() bool {
                                   ctnrRoutes := []*routes.ContainerRoute{}
                                   for _, container := range g.RealState.Containers(val.AppName) {
                                          if container.ID == val.ContainerID {
                                                 switch val.OperateType {
                                                 case FS.Normal, FS.Restart:
                                                        for _, p := range container.PortInfos {
                                                               if p.Ports == nil || len(p.Ports) == 0 {
                                                                      return false
                                                               }
                                                        }
                                                 case FS.Maint:
                                                        for _, p := range container.PortInfos {
                                                               if p.Ports != nil || len(p.Ports) != 0 {
                                                                      return false
                                                               }
                                                        }
                                                 }
                                          }
                                          ctnrRoute := routes.GenerateRouteData(container)
                                          if ctnrRoute != nil {
                                                 ctnrRoutes = append(ctnrRoutes, ctnrRoute)
                                          }
                                   }
								   if err := rt.routerDrv.UpdateRoutes(val.AppName, ctnrRoutes); nil != err {
                                          glog.Errorf("UpdateRoutes failed. Error: %v", err)
                                   }
                                   return true
                            }
 
                            for update() == false && time.Since(val.Birthday) < time.Second*3*time.Duration(g.Config().Interval) {
                                   time.Sleep(time.Second)
                            }
                     })
              }
       }
}

func (rt *Route) registerEventsToME() error {
       userstarted := &ME.User{
              EventID:     ME.AppStatusToStarted,
              RequireData: true,
              Drop:        false,
              Description: "router discover model",
       }
       if _, err := ME.RegisterEvent(userstarted, &rt.appStartedChan); err != nil {
              return err
       }
 
       userdelapp := &ME.User{
              EventID:     ME.DeleteContainersBecauseTheAppItBelongsWasDeleted,
              RequireData: true,
              Drop:        false,
              Description: "router discover model",
       }
       if _, err := ME.RegisterEvent(userdelapp, &rt.appDeletedChan); err != nil {
              return err
       }
	   userfailover := &ME.User{
              EventID:     ME.Failover,
              RequireData: true,
              Drop:        false,
              Description: "router discover model",
       }
       if _, err := ME.RegisterEvent(userfailover, &rt.failoverChan); err != nil {
              return err
       }
 
       userfastsetting := &ME.User{
              EventID:     ME.FastSettingSuccess,
              RequireData: true,
              Drop:        false,
              Description: "router discover model",
       }
       if _, err := ME.RegisterEvent(userfastsetting, &rt.fastSettingSuccessChan); err != nil {
              return err
       }
       return nil
}
