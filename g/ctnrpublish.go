package g
 
import (
       "encoding/json"
       "fmt"
       ctnrpub "github-beta.huawei.com/hipaas/common/ctnrpublish"
       FS "github-beta.huawei.com/hipaas/common/fastsetting"
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       TP "github-beta.huawei.com/hipaas/common/util/taskpool"
       "github-beta.huawei.com/hipaas/glog"
       "reflect"
       "time"
)
 
var (
       ctnrPublisher ContainerPublish
)

/*
       Description:  start container publish model driver with a task controll pool.
*/
func StartCtnrPublisher() error {
       cfg := ctnrpub.ConsulPubConfig{
              Address:        Config().ConsulServer.ConsulAddress,
              Scheme:         Config().ConsulServer.ConsulScheme,
              Datacenter:     Config().ConsulServer.ConsulDatacenter,
              Token:          Config().ConsulServer.CtnrToken,
              ConsulCtnrPath: Config().ConsulServer.ConsulCtnrPath,
       }
       driver, err := ctnrpub.NewCtnrPubDrv(cfg)
       if nil != err {
              return fmt.Errorf("Create container publish driver failed. Error: %v", err)
       }
       pool, poolerr := TP.NewTaskPool(200)
       if nil != poolerr {
              return fmt.Errorf("create task pool failed. Error: %v", poolerr)
       }
	   ctnrPublisher = ContainerPublish{
              drv:                    driver,
              task:                   pool,
              appStartedChan:         make(chan []byte, 200),
              delCtnrChan:            make(chan []byte, 200),
              fastSettingSuccessChan: make(chan []byte, 200),
       }
       // regiser events
       if err := ctnrPublisher.registerCtnrPublisherEvents(); nil != err {
              return fmt.Errorf("register events to message engine failed. Error: %v", err)
       }
       go ctnrPublisher.eventCollector()
       // update app containers in loop
       go ctnrPublisher.updateAllCtnrs(Config().ConsulServer.UpdateCtnrInterval)
       return nil
}
 
type ContainerPublish struct {
       drv                    ctnrpub.CtnrPubManager
       task                   TP.TaskPoolInterface
       appStartedChan         chan []byte
       delCtnrChan            chan []byte
       fastSettingSuccessChan chan []byte
}

/*
       Description:  this is a go routine to syncronize containers info in consul
       with local container data.
*/
func (cp *ContainerPublish) updateAllCtnrs(interval int) {
       duration := time.Duration(interval) * time.Second
       for {
              time.Sleep(duration)
              // get all the apps in status pending, created and started.
              apps, err := GetAppByStatus(app.AppStatusPending, app.AppStatusCreateSuccess, app.AppStatusStartSuccess)
              if nil != err {
                     glog.Error("GetAppByStatus failed. Error: ", err)
                     continue
              }
              // list all the apps in consul
              appsmap, err := cp.drv.ListApps()
              if nil != err {
                     glog.Error("container publish ListApps failed. Error:", err)
                     continue
              }
			  for appName, _ := range appsmap {
                     //delete the containers do not exist in realstate
                     if _, ok := apps[appName]; !ok {
                            cp.task.Require()
                            go func(name string) {
                                   defer cp.task.Release()
                                   if err = cp.drv.DelAppCtnrs(name); nil != err {
                                          glog.Error("DelAppCtnrs failed. Error: ", err)
                                   } else {
                                          glog.Warningf("Delete app: %s from consul which is does not exist in redis.", name)
                                   }
                            }(appName)
                     }
              }
              for appname, realapp := range apps {
                     //only add started success containers
                     if realapp.Status == app.AppStatusStartSuccess {
					 cp.task.Require()
                            go func(appname string) {
                                   defer cp.task.Release()
                                   cnslCtnrs, err := cp.drv.GetAppCtnrs(appname)
                                   if err != nil {
                                          glog.Errorf("Get app containers failed. Error: %v", err)
                                          return
                                   }
                                   realCtnrs := RealState.Containers(appname)
                                   var ctnrs []realstate.Container
                                   for _, ctnr := range realCtnrs {
                                          // UpdateAt and Status don't need to be published
										  ctnr.UpdateAt = 0
                                          ctnr.Status = ""
                                          ctnrs = append(ctnrs, *ctnr)
                                   }
 
                                   //only update changed apps
                                   realEnv := make(map[string]string)
                                   realPort := make(map[string]int)
                                   realPortInfo := make(map[string]string)
 
                                   cnslEnv := make(map[string]string)
                                   cnslPort := make(map[string]int)
                                   cnslPortInfo := make(map[string]string)
 
                                   realMap := make(map[string]realstate.Container, len(realCtnrs))
								   for _, ctnr := range ctnrs {
                                          for _, env := range ctnr.Envs {
                                                 realEnv[ctnr.ID+env.K] = env.V
                                          }
                                          for _, port := range ctnr.Ports {
                                                 strPort := fmt.Sprintf("%d", port.PublicPort)
                                                 realPort[ctnr.ID+strPort] = port.PublicPort
                                          }
                                          for _, portInfo := range ctnr.PortInfos {
                                                 var info string
                                                 if len(portInfo.Ports) > 0 {
                                                        info = fmt.Sprintf("portname:%v HostIP:%v HostPort:%v originport:%v",portInfo.Portname, portInfo.Ports[0].HostIP, portInfo.Ports[0].HostPort, portInfo.OriginPort)
                                                 } else {
                                                        info = fmt.Sprintf("portname:%v originport:%v", portInfo.Portname, portInfo.OriginPort)
                                                 }
                                                 realPortInfo[info] = ""
                                          }
 
                                          ctnr.Envs = nil
                                          ctnr.Ports = nil
                                          ctnr.PortInfos = nil
                                          realMap[ctnr.ID] = ctnr
                                   }
								   cnslMap := make(map[string]realstate.Container, len(*cnslCtnrs))
                                   for _, ctnr := range *cnslCtnrs {
                                          for _, env := range ctnr.Envs {
                                                 cnslEnv[ctnr.ID+env.K] = env.V
                                          }
 
                                          for _, port := range ctnr.Ports {
                                                 strPort := fmt.Sprintf("%d", port.PublicPort)
                                                 cnslPort[ctnr.ID+strPort] = port.PublicPort
                                          }
 
                                          for _, portInfo := range ctnr.PortInfos {
										  var info string
                                                 if len(portInfo.Ports) > 0 {
                                                        info = fmt.Sprintf("portname:%v HostIP:%v HostPort:%v originport:%v",
                                                               portInfo.Portname, portInfo.Ports[0].HostIP, portInfo.Ports[0].HostPort, portInfo.OriginPort)
                                                 } else {
                                                        info = fmt.Sprintf("portname:%v originport:%v", portInfo.Portname, portInfo.OriginPort)
                                                 }
                                                 cnslPortInfo[info] = ""
                                          }
 
                                          ctnr.Envs = nil
                                          ctnr.Ports = nil
										  ctnr.PortInfos = nil
                                          cnslMap[ctnr.ID] = ctnr
                                   }
 
                                   if reflect.DeepEqual(realMap, cnslMap) &&
                                          reflect.DeepEqual(realEnv, cnslEnv) &&
                                          reflect.DeepEqual(realPort, cnslPort) &&
                                          reflect.DeepEqual(realPortInfo, cnslPortInfo) {
                                          return
                                   }
 
                                   glog.Debug("sync update containers pub, appname", appname)
                                   if err = cp.drv.UpdateAppCtnrs(appname, &ctnrs); nil != err {
                                          glog.Error("UpdateAppCtnrs failed. Error: ", err)
										  } else {
                                          glog.Infof("Update app: %v containers to consul", appname)
                                   }
                            }(appname)
                     }
              }
       }
}
 
/*
       Description: register all the needed events to message engine. so that
       we can received these events.
*/
func (cp *ContainerPublish) registerCtnrPublisherEvents() error {
       appStartedUser := ME.User{
              EventID:     ME.AppStatusToStarted,
              RequireData: true,
              Drop:        false,
              Description: "ctnr publisher.",
       }
	   if _, err := ME.RegisterEvent(&appStartedUser, &cp.appStartedChan); nil != err {
              return fmt.Errorf("regisger ctnr publisher failed. event: AppStatusToStarted, Error: %v", err)
       }
 
       delCtnrUser := ME.User{
              EventID:     ME.DeleteContainersBecauseTheAppItBelongsWasDeleted,
              RequireData: true,
              Drop:        false,
              Description: "ctnr publisher.",
       }
       if _, err := ME.RegisterEvent(&delCtnrUser, &cp.delCtnrChan); nil != err {
              return fmt.Errorf("regisger ctnr publisher failed. event: DeleteContainersBecauseTheAppItBelongsWasDeleted, Error: %v", err)
       }
	   fastSettingSuccessUser := ME.User{
              EventID:     ME.FastSettingSuccess,
              RequireData: true,
              Drop:        false,
              Description: "ctnr publisher.",
       }
       if _, err := ME.RegisterEvent(&fastSettingSuccessUser, &cp.fastSettingSuccessChan); nil != err {
              return fmt.Errorf("regisger ctnr publisher failed. event: FastSettingSuccess, Error: %v", err)
       }
       return nil
}
 
/*
       Description: wait for receiving events from message engine in a infinite for loop.
       we can not ensure that all the event are received in time series. but this do assure
       that each type of events are received in time series.
*/
func (cp *ContainerPublish) eventCollector() {
       for {
              select {
              case dat := <-cp.appStartedChan:
                     val := ME.AppStatusToStartedData{}
                     if err := json.Unmarshal(dat, &val); nil != err {
                            glog.Error("ctnr pub event AppStatusToStartedData unamrshal failed. Error: ", err)
                            continue
                     }
                     cp.task.Execute(func() {
                            // UpdateAt and Status don't need to be published
                            for k, _ := range val.Containers {
                                   val.Containers[k].UpdateAt = 0
                                   val.Containers[k].Status = ""
                            }
                            if err := cp.drv.AddAppCtnrs(val.App.Name, &val.Containers); nil != err {
									glog.Errorf("Update app %v containers failed. Error: %v", val.App.Name, err)
                            } else {
                                   glog.Infof("Update app %s containers to consul.", val.App.Name)
                            }
                     })
 
              case dat := <-cp.delCtnrChan:
                     val := ME.DeleteContainersBecauseTheAppItBelongsWasDeletedData{}
                     if err := json.Unmarshal(dat, &val); nil != err {
                            glog.Error("ctnr pub event DeleteContainersBecauseTheAppItBelongsWasDeletedData unamrshal failed. Error: ", err)
                            continue
                     }
					 cp.task.Execute(func() {
                            if err := cp.drv.DelAppCtnrs(val.AppName); nil != err {
                                   glog.Errorf("delete app %v containers from consul failed. Error: %v", val.AppName, err)
                            } else {
                                   glog.Infof("delete app %s containers from consul.", val.AppName)
                            }
                     })
 
              case dat := <-cp.fastSettingSuccessChan:
                     val := ME.FastSettingSuccessData{}
                     if err := json.Unmarshal(dat, &val); nil != err {
                            glog.Error("ctnr pub event FastSettingSuccessData unamrshal failed. Error: ", err)
                            continue
                     }
					 cp.task.Execute(func() {
                            update := func() bool {
                                   containers := RealState.Containers(val.AppName)
                                   ctnrs := make([]realstate.Container, 0)
                                   for _, container := range containers {
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
                                          // UpdateAt and Status don't need to be published
										  container.UpdateAt = 0
                                          container.Status = ""
                                          ctnrs = append(ctnrs, *container)
                                   }
                                   if err := cp.drv.AddAppCtnrs(val.AppName, &ctnrs); nil != err {
                                          glog.Errorf("Update app %v containers failed. Error: %v", val.AppName, err)
                                   } else {
                                          glog.Infof("Update app %s containers to consul.", val.AppName)
                                   }
                                   return true
                            }
							for update() == false && time.Since(val.Birthday) < time.Second*3*time.Duration(Config().Interval) {
                                   time.Sleep(time.Second)
                            }
                     })
              }
 
       }
}
