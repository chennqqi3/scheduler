package g
 
import (
       "errors"
)
 
import (
       "encoding/json"
       "fmt"
       "github.com/hashicorp/consul/api"
       "strconv"
       "strings"
       "time"
 
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       "github-beta.huawei.com/hipaas/common/service"
       appstorage "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       TP "github-beta.huawei.com/hipaas/common/util/taskpool"
       "github-beta.huawei.com/hipaas/glog"
)
 
/*
       Model Description:  this is a container service discovery model, which is to
       execute container discover service when the app (the coantiner belongs.)
status turns to started.
              This model will call the consul agent, where the container is running on,
       to register the container to consul cluster. It's convenient to register con-
       taine to different consul clusters with varies "Data Center" (simplied called
       "DC")in this way. A container's DC is decided by the consul agent's DC where
       it's registered.
 
       Note 1: only containers with a port named "monitor" will be registered for di-
       scovering.
       Note 2: this service works for prometheus container monitoring.
       Note 3: only "LRP" and "OOT" type of app will be handled.
*/
 
const (
       // a port named as "monitor" will proceed service regist service with consul
       appPortType string = "monitor"
       envName     string = "envname"
       // consul agent http port.
       consulPort  int    = 8500
)

var csd CtnrSvcDiscv
 
func StartService() error {
       pool, poolerr := TP.NewTaskPool(200)
       if nil != poolerr {
              return fmt.Errorf("create task pool failed. Error: %v", poolerr)
       }
       csd = CtnrSvcDiscv{
              Task:           pool,
              PortType:       appPortType,
              Prefix:         Config().ConsulService.CtnrServicePrefix,
              RegEventChan:   make(chan []byte, 300),
              DeregEventChan: make(chan []byte, 300),
       }
       if err := csd.RegisterEventsToME(); err != nil {
              return errors.New("service discovery register to events center error: " + err.Error())
       }
       go csd.WaitEvent()
       go csd.ctnrServiceGuard()
       return nil
}

type CtnrSvcDiscv struct {
       Task               TP.TaskPoolInterface
       PortType           string
       Prefix             string
       RegEventChan       chan []byte
       DeregEventChan     chan []byte
       eventUsrAppStartSn int64
       eventDeleteCtnrSn  int64
}
 
/*
       Description:  newDrv method generate a new consul connection drive for each
       container, which is used to register the container service to consul cluster.
*/
func (cs *CtnrSvcDiscv) newDrv(address string) (*service.CtnrService, error) {
       client, err := service.NewClient(api.Config{
              Address: fmt.Sprintf("%s:%d", address, consulPort),
              Scheme:  "http",
       })
       if nil != err {
              return nil, fmt.Errorf("start consul client error: %v", err)
       }
       c := Config()
       cfg := &service.CtnrConsulCfg{
              Prefix: c.ConsulService.CtnrServicePrefix,
              Tags:   c.ConsulService.CtnrServiceTags,
       }

// Instantiate cosul connection objects
       drv, hasErr := service.NewCtnrService(client, cfg)
       if nil != hasErr {
              return nil, fmt.Errorf("NewCtnrService error: %v", hasErr)
       }
       return drv, nil
}
 
/*
       Description:  containers with apptype like "LRP" and "OOT", and with port
       named with "monitor" is a legal container that have service operation.
*/
func (cs *CtnrSvcDiscv) ctnrFilter(ctnr *realstate.Container) bool {
       if ctnr.AppType != appstorage.AppTypeLRC && ctnr.AppType != appstorage.AppTypeSRC {
              return false
       }
       for _, portinfo := range ctnr.PortInfos {
              if portinfo.Portname == cs.PortType {
                     return true
              }
       }
       return false
}

/*
       Description:  register container to consul cluster. each register operation
       is handled with a independent go routine.
*/
func (cs *CtnrSvcDiscv) register(ctnr realstate.Container) {
       // only lrp and oot type of app will be registered.
       if ctnr.AppType != appstorage.AppTypeLRC && ctnr.AppType != appstorage.AppTypeSRC {
              return
       }
       cs.Task.Execute(func() {
              // serviceEnv is used to tag this container.
              serviceEnv := ""
              for _, v := range ctnr.Envs {
                     if strings.ToLower(v.K) == envName {
                            serviceEnv = v.V
                     }
              }
              registerPort := func(port int) {
                     drv, err := cs.newDrv(ctnr.IP)
                     if nil != err {
                            glog.Errorf("init service driver failed. Error: %v", err)
                            return
                     }
					 if err := drv.RegisterCtnr(ctnr.AppName, ctnr.IP, ctnr.MinionHostname, ctnr.ID, port, serviceEnv); nil != err {
                            glog.Errorf("Register container id: %s, ip: %s failed. Error: %s", ctnr.ID, ctnr.IP, err.Error())
                     } else {
                            glog.Infof("Register container id: %s, ip: %s to consul", ctnr.ID, ctnr.IP)
                     }
              }
 
              for _, portinfo := range ctnr.PortInfos {
                     if portinfo.Portname == cs.PortType {
                            // containers with no port being mounted, the Ports field will be set to "-1".
                            if len(portinfo.Ports) < 1 {
                                   registerPort(-1)
                            } else {
								for _, dockerPort := range portinfo.Ports {
                                          if "" == dockerPort.HostPort {
                                                 continue
                                          }
                                          if port, err := strconv.Atoi(dockerPort.HostPort); nil != err {
                                                 glog.Error("ports.HostPort atoi failed. Ctnr:", ctnr)
                                          } else {
                                                 registerPort(port)
                                                 break
                                          }
                                   }
                            }
                            break
                     }
              }
       })
}
 
/*
       Description:  deregister container from consul when container is deleted.
*/
func (cs *CtnrSvcDiscv) deregister(id string, ip string) {
       if "" == id || "" == ip {
              glog.Errorf(" deregister container failed. Error:invalid input parameter.")
              return
       }
       cs.Task.Execute(func() {
              drv, err := cs.newDrv(ip)
              if nil != err {
                     glog.Errorf("init service driver failed. Error: %v", err)
                     return
              }
 
              if err = drv.DeRegisterCtnr(id); nil != err {
                     glog.Errorf("DeRegister container id: %s, ip: %s failed. Error: %s", id, ip, err.Error())
              } else {
                     glog.Infof("Deregister container id: %s, ip: %s from consul", id, ip)
              }
       })
}
 
/*
       Deregister:  register needed events to message engine.
*/
func (cs *CtnrSvcDiscv) RegisterEventsToME() error {
       userstarted := &ME.User{
              EventID:     ME.AppStatusToStarted,
              RequireData: true,
              Drop:        false,
              Description: "container service discover model",
       }
       var err error
       cs.eventUsrAppStartSn, err = ME.RegisterEvent(userstarted, &cs.RegEventChan)
       if nil != err {
              return err
       }
       glog.Info("register event AppStatusToStarted to message engine succeed.")
 
       userdelctnr := &ME.User{
              EventID:     ME.DeleteOneContainerForAnyReason,
              RequireData: true,
              Drop:        false,
              Description: "container service discover model",
       }
	   cs.eventDeleteCtnrSn, err = ME.RegisterEvent(userdelctnr, &cs.DeregEventChan)
       if nil != err {
              return err
       }
       glog.Info("register event DeleteContainersBecauseTheAppItBelongsWasDeleted to message engine succeed.")
       return nil
}
 
/*
       Description:  wait for receiving events form message engine. A register or
       deregister operation is done when an event is received.
*/
func (cs *CtnrSvcDiscv) WaitEvent() {
       for {
              select {
              case dat := <-cs.RegEventChan:
                     val := ME.AppStatusToStartedData{}
                     if err := json.Unmarshal(dat, &val); nil != err {
                            glog.Error(err)
                            continue
                     }
                     // glog.Debug("ctnr service received register event: ", string(dat))
					 // register container(s) the app have.
                     for _, ctnr := range val.Containers {
                            if cs.ctnrFilter(&ctnr) {
                                   cs.register(ctnr)
                            }
                     }
              case dat := <-cs.DeregEventChan:
                     val := ME.DeleteOneContainerForAnyReasonData{}
                     if err := json.Unmarshal(dat, &val); nil != err {
                            glog.Error(err)
                            continue
                     }
                     // glog.Debug("ctnr service received deregister event: ", string(dat))
                     // deregister container(s) the app have.
                     cs.deregister(val.Container.ID, val.Container.IP)
              }
       }
}


/*
       Description:  a goroutine with a infinite for loop to handle synchronise
       containers in local redis and remote consul cluster. Containers only exist
       in local redis will be register to consul, and containers only exist in
       remote consul cluster will be desgister from consul.
*/
func (cs *CtnrSvcDiscv) ctnrServiceGuard() {
       duration := time.Duration(300) * time.Second
       msduration := time.Duration(100) * time.Millisecond
       for {
              time.Sleep(duration)
 
              // get all the apps in mysql.
              apps, err := GetDesiredState()
              if err != nil {
                     glog.Errorf("GetDesiredState error: %s", err.Error())
                     continue
              }
			  ctnrAppStatusMap := make(map[string]string)
              nodeCtnrs := make(map[string][]*realstate.Container)
              for _, app := range apps {
                     // get all the app's containers in local redis.
                     containers := RealState.Containers(app.Name)
                     for _, ctnr := range containers {
                            nodeCtnrs[ctnr.IP] = append(nodeCtnrs[ctnr.IP], ctnr)
                            ctnrAppStatusMap[ctnr.ID] = app.Status
                     }
              }
 
              // get all the working minions in this hipaas cluster.
              allNodes, err := RealNodeState.GetAllNode()
              if err != nil {
                     glog.Errorf("get all nodes error: %s", err.Error())
                     continue
              }
 
              // handle each minions with a for loop.
              for _, node := range allNodes {
                     svcmap, err := cs.getServiceIDMap(node.IP)
					 if nil != err {
                            glog.Error("getServiceIDMap failed. Error: ", err)
                            continue
                     }
 
                     containers, ok := nodeCtnrs[node.IP]
                     if ok {
                            for _, ctnr := range containers {
                                   if _, ok := svcmap[ctnr.ID]; ok {
                                          delete(svcmap, ctnr.ID)
                                   } else if ctnrAppStatusMap[ctnr.ID] == appstorage.AppStatusStartSuccess {
                                          if cs.ctnrFilter(ctnr) {
                                                 cs.register(*ctnr)
                                                 glog.Warningf("Sync total register container id: %s, ip: %s to consul", ctnr.ID, ctnr.IP)
}
                                   }
                            }
                     }
                     for id, ip := range svcmap {
                            cs.deregister(id, ip)
                            glog.Warningf("Sync total deregister container id: %s, ip: %s from consul", id, ip)
                     }
 
                     // handle one minion every msduration
                     time.Sleep(msduration)
              }
       }
}
 
/*
       Description:  get services in each minion.
*/
func (cs *CtnrSvcDiscv) getServiceIDMap(ip string) (map[string]string, error) {
       // get the connection with the minion/consul agent.
       drv, err := cs.newDrv(ip)
       if nil != err {
              return nil, err
       }
 
       // get services registered in this conaul agent.
       services, hasErr := drv.Services()
       if nil != hasErr {
              return nil, hasErr
       }
	   svcmap := make(map[string]string)
       for _, svcs := range services {
              if strings.HasPrefix(svcs.ID, cs.Prefix) {
                     svcmap[svcs.ID[len(cs.Prefix):]] = svcs.Address
              }
       }
       return svcmap, nil
}


