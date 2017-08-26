package sync
 
import (
       "container/list"
       "encoding/json"
       "errors"
       "fmt"
       "io/ioutil"
       "net/http"
       "strings"
       "time"
 
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       appStorage "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/common/util/uuid"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/scheduler/executor"
       "github-beta.huawei.com/hipaas/scheduler/fastsetting"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm"
)

// container
const imageRunMiniTime = 60
 
//LRP container
type ContainerManager struct {
       SyncController
}
 
func NewContainerManager(
       realState g.ContainerRealState,
       creatingState realstate.DockerRunStatusProvider,
       nodes g.MinionStorageDriver,
       config ConfigFunc,
       algo *algorithm.Config,
       updateAppStatusByName UpdateAppStatusByNameFunc,
	   ) SchedulerProvider {
       return &ContainerManager{
              SyncController: SyncController{
                     RealState:     realState,
                     CreatingState: creatingState,
                     Nodes:         nodes,
                     Config:        config,
                     Algo:          algo,
                     UpdateAppStatusByNameFunc: updateAppStatusByName,
              },
       }
}

func (c *ContainerManager) StatusCompare(app *appStorage.App) error {
       if app == nil {
              return errors.New("app is nil")
       }
 
       //Status Arrangement
       ctnrUpCount := 0
       containers := c.RealState.Containers(app.Name)
 
       for _, container := range containers {
              if UpdateCompareFunc(app, container) == false {
                     ctnrUpCount  
              }
       }
 
       //creating status compare
       glog.Info(app.Name, ", RealCount: ", ctnrUpCount, ", DesiredCount: ", app.Instance)
       finish := func(status *realstate.DockerStatusStore) error {
              for _, node := range status.Node {
                     virNode, exist := g.RealNodeState.GetNode(node.IP)
                     if !exist {						
                            glog.Warningf("virNode is not exist, IP: %s", node.IP)
							continue
							
                     }
                     virNode.CPUVirtUsage = virNode.CPUVirtUsage + node.Cpu
                     virNode.MemVirtUsage = virNode.MemVirtUsage + node.Memory
                     g.RealNodeState.UpdateNode(virNode)
              }
              return nil
       }
       err := c.CreatingStatus(app.Name, app.Instance, 0, func() error { return nil }, finish)
       if err != nil {
              return err
       }
       if ctnrUpCount == app.Instance && len(containers) == app.Instance {
              glog.Infof("status compare success, name: %s, update status to %s", app.Name, appStorage.AppStatusCreateSuccess)
              c.UpdateAppStatus(app, appStorage.AppStatusCreateSuccess, "")
       }
 
       return nil
}

func (c *ContainerManager) CreateContainers(app *appStorage.App) error {
       if app.Status != appStorage.AppStatusPending {
              err := c.UpdateAppStatus(app, appStorage.AppStatusPending, "")
              if err != nil {
                     glog.Error("UpdateAppStatus fail: ", err)
                     return err
              }
       }
       err := c.createSomeContainers(app, app.Instance)
       if err != nil {
              return err
       }
       return nil
}
 
func (c *ContainerManager) createSomeContainers(app *appStorage.App, num int) error {
       callBack := func() error {
              if num == 0 {
                     return nil
              }
              glog.Infof("CreateContainers, name: %s, num: %d", app.Name, num)
              now := time.Now()
              ipCount, err := c.Algo.Algorithm.Schedule(app, num)
			  glog.Infof("schedule task name: %s, spend %v", app.Name, time.Now().Sub(now))
              if err != nil {
                     glog.Error("Algorithm.Schedule error: ", err)
                     c.UpdateAppStatus(app, appStorage.AppStatusPending, err.Error())
                     ME.NewEventReporter(ME.ExecResult, ME.ExecResultData{
                            Reason:   "Algorithm.Schedule error",
                            AppName:  app.Name,
                            Error:    err.Error(),
                            Birthday: time.Now(),
                            Message:  fmt.Sprintf("Scheduler AppName %v Fail --> %v", app.Name, err),
                     })
                     return err
              }
              if num == app.Instance {
                     ME.NewEventReporter(ME.StartToCreateApp,
                            ME.StartToCreateAppData{
                                   App:      *app,
								   Birthday: time.Now(),
                                   Message:  fmt.Sprintf("App %v Begin to Create %v Containers --> %v", app.Name, app.Instance, app.Image.DockerImageURL),
                            })
              }
              c.runContainers(app, ipCount)
              return nil
       }
       finish := func(status *realstate.DockerStatusStore) error {
              for _, node := range status.Node {
                     virNode, exist := g.RealNodeState.GetNode(node.IP)
                     if !exist {
                            glog.Warningf("virNode is not exist, IP: %s", node.IP)
                            continue
                     }
                     virNode.CPUVirtUsage = virNode.CPUVirtUsage + node.Cpu
                     virNode.MemVirtUsage = virNode.MemVirtUsage + node.Memory
                     g.RealNodeState.UpdateNode(virNode)
              }
              return nil
       }
return c.CreatingStatus(app.Name, app.Instance, num, callBack, finish)
}
 
type delPriority int
 
const (
       delNone      delPriority = 0
       inDesignated delPriority = 1
       notUpStatus  delPriority = 2
       upStatus     delPriority = 3
       protected    delPriority = 4
)
 
func (c *ContainerManager) getDeletePriority(ctnr *realstate.Container, mapCtnrsShrinked map[string]bool) delPriority {
       //protected
       fastsettingInstance, err := fastsetting.GetFastsetting()
       if nil != err {
              return delNone
       }
	   exist, err := fastsettingInstance.IsCTNRExecModeExist(ctnr.ID)
       if nil != err {
              glog.Errorf("IsCTNRExecModeExist: %s fail:%v", ctnr.ID, err)
              return delNone
       }
       if exist {
              return protected
       }
       // in designated
       if _, exist := mapCtnrsShrinked[ctnr.ID]; exist {
              return inDesignated
       }
       // up status
       if strings.Contains(ctnr.Status, realstate.ContainerStatusUp) {
              return upStatus
       }
       // not up status
       return notUpStatus
}
 
func (c *ContainerManager) chooseDeleteCtnrs(appName string, ctnrs []*realstate.Container, delCount int) ([]*realstate.Container, error) {
       jobContents, err := g.GetShrinkedTask(appName, appStorage.JobStatusDefined)
       if err != nil {
              return nil, err
       }
	   mapCtnrsShrinked := make(map[string]bool)
       for _, jobContent := range jobContents {
              var opt *appStorage.UpdateInstanceOption
              err = json.Unmarshal([]byte(jobContent), &opt)
              if err != nil {
                     return nil, err
              }
              for _, ctnrID := range opt.ContainerIDs {
                     mapCtnrsShrinked[ctnrID] = true
              }
       }
 
       delCtnrsList := list.New()
       var eleInDesignated, eleNotUpStatus, eleUpStatus *list.Element
       for _, ctnr := range ctnrs {
              switch c.getDeletePriority(ctnr, mapCtnrsShrinked) {
              case inDesignated:
                     if eleInDesignated != nil {
                            eleInDesignated = delCtnrsList.InsertAfter(ctnr, eleInDesignated)
                            break
                     }
					 eleInDesignated = delCtnrsList.PushFront(ctnr)
              case notUpStatus:
                     if eleNotUpStatus != nil {
                            eleNotUpStatus = delCtnrsList.InsertAfter(ctnr, eleNotUpStatus)
                            break
                     }
                     if eleInDesignated != nil {
                            eleNotUpStatus = delCtnrsList.InsertAfter(ctnr, eleInDesignated)
                            break
                     }
                     eleNotUpStatus = delCtnrsList.PushFront(ctnr)
              case upStatus:
                     if eleUpStatus != nil {
                            eleUpStatus = delCtnrsList.InsertAfter(ctnr, eleUpStatus)
                            break
                     }
                     if eleNotUpStatus != nil {
                            eleUpStatus = delCtnrsList.InsertAfter(ctnr, eleNotUpStatus)
                            break
                     }
					 if eleInDesignated != nil {
                            eleUpStatus = delCtnrsList.InsertAfter(ctnr, eleInDesignated)
                            break
                     }
                     eleUpStatus = delCtnrsList.PushFront(ctnr)
              case protected:
                     delCtnrsList.PushBack(ctnr)
              case delNone:
                     // redis error, nothing to do
              }
       }
 
       delCtnrs, length := []*realstate.Container{}, 0
       for ele := delCtnrsList.Front(); ele != nil; ele = ele.Next() {
              value, _ := ele.Value.(*realstate.Container)
              delCtnrs = append(delCtnrs, value)
              length  
              if length >= delCount {
                     break
              }
       }
       return delCtnrs, nil
}

func (c *ContainerManager) UpdateContainers(app *appStorage.App) error {
       realCount := c.RealState.ContainerCount(app.Name)
       if app.Instance == realCount {
              return nil
       }
       if app.Status != appStorage.AppStatusPending {
              err := c.UpdateAppStatus(app, appStorage.AppStatusPending, "")
              if err != nil {
                     glog.Error("UpdateAppStatus fail: ", err)
                     return err
              }
       }
       ME.NewEventReporter(ME.UpdateApp, ME.UpdateAppData{
              App:      *app,
              Birthday: time.Now(),
              Message:  fmt.Sprintf("Update App %s instance  --> from %d to %d", app.Name, realCount, app.Instance),
       })
	   glog.Infof("update %s containers from instance %d to %d", app.Name, realCount, app.Instance)
       if app.Instance > realCount {
              diffCount := app.Instance - realCount
              err := c.createSomeContainers(app, diffCount)
              return err
       } else {
              glog.Warningf("DropContainers, because users modify the configuration, name: %s, desiredcount: %d", app.Name, app.Instance)
              diffCount := realCount - app.Instance
              containers, err := c.chooseDeleteCtnrs(app.Name, g.RealState.Containers(app.Name), diffCount)
              if err != nil {
                     glog.Errorf("chooseDeleteCtnrs fail: %v", err)
                     return err
              }
              c.DropContainers(app.Name, app.Instance, containers)
       }
       return nil
}

func (c *ContainerManager) DropContainers(appName string, appInstance int, containers []*realstate.Container) error {
       callBack := func() error {
              glog.Warningf("DropContainers, name: %s, num: %d", appName, len(containers))
              err := c.CreatingState.DockerStatusUpdate(
                     appName, len(containers),
                     nil, StatusDeleteTimeOut, StatusDeleteTimeAgeing)
              if err != nil {
                     glog.Error("CreatingState.DockerStatusUpdate fail: ", err)
                     return err
              }
              for _, container := range containers {
                     c.DropOneContainer(container)
              }
              return nil
       }
	   finish := func(status *realstate.DockerStatusStore) error {
              return nil
       }
       err := c.CreatingStatus(appName, appInstance, len(containers), callBack, finish)
       return err
}
 
func (c *ContainerManager) runContainers(app *appStorage.App, ipCount map[string]int) {
       name := func() string {
              if id := uuid.GetGuid(); 13 <= len(id) {
                     return id[0:13]   "_"   app.Name
              } else {
                     return id   "_"   app.Name
              }
       }
       for selectNodeIP, count := range ipCount {
              nodeAppDeployInfo := &node.NodeAppDeployInfo{App: app}
              for k := 0; k < count; k   {
                     deployInfo := &node.DeployInfo{
                            AgentIP:  selectNodeIP,
                            Ctnrname: name(),
                     }
					 nodeAppDeployInfo.DeployInfos = append(nodeAppDeployInfo.DeployInfos, deployInfo)
              }
              go executor.GetDockerManager().NodeTaskCreateCtnr(selectNodeIP, []*node.NodeAppDeployInfo{nodeAppDeployInfo})
       }
}
 
func (c *ContainerManager) RequireResource(app *appStorage.App) error {
       err := c.UpdateAppStatus(app, appStorage.AppStatusPending, "")
       if err != nil {
              return err
       }
       return nil
}
 
func (c *ContainerManager) healthCheckOneContainer(container *realstate.Container) (ok bool) {
       defer func() {
              if ok {
                     if err := g.HealthCheckTries.Delete(container); err != nil {
                            glog.Errorf("delete health check tries error: %s", err.Error())
                     }
                     return
              }
			  tries, err := g.HealthCheckTries.Increase(container)
              if err != nil {
                     glog.Errorf("increase health check tries error: %s", err.Error())
                     return
              }
              if tries > c.Config().Health.MaxTries {
                     errInfo := fmt.Sprintf("delete container due to health check failed to many times, id: %s, appname: %s, ip: %s",
                            container.ID, container.AppName, container.IP)
                     ME.NewEventReporter(ME.ExecResult, ME.ExecResultData{
                            Reason:   "app health check failed.",
                            AppName:  container.AppName,
                            Error:    errInfo,
							Birthday: time.Now(),
                            Message:  "health check failed over the max tries.",
                     })
                     c.UpdateAppStatusByNameFunc(container.AppName, appStorage.AppStatusCreateSuccess, errInfo)
                     if err := g.HealthCheckTries.Delete(container); err != nil {
                            glog.Errorf("delete health check tries error: %s", err.Error())
                     }
                     c.DropOneContainer(container)
                     glog.Errorf(errInfo)
              }
       }()
 
       switch container.HealthMode {
       case realstate.CTNR_HEALTH_ALIVE:
              return true
       case realstate.CTNR_HEALTH_UNKNOWN:
              glog.Warningf("health check ctnrid:%s ;appname:%s ; ip:%s ; status unknown. ",
                     container.ID, container.AppName, container.IP)
              return false
			  case realstate.CTNR_HEALTH_DEAD:
              glog.Errorf("health check container failed. ctnrid:%s ;appname:%s ; ip:%s ;ports:%v ",
                     container.ID, container.AppName, container.IP, container.PortInfos)
              return false
       default:
              glog.Errorf(" health check mode:%v does not exist. ctnrid:%s, ip:%s",
                     container.HealthMode, container.ID, container.IP)
              return false
       }
 
}
 
func (c *ContainerManager) HealthCheck(app *appStorage.App) error {
       if !c.RealState.RealAppExists(app.Name) {
              return fmt.Errorf("real state do not exist for app: %s", app.Name)
       }
	   if app.Status == appStorage.AppStatusCreateSuccess || app.Status == appStorage.AppStatusStartSuccess {
              containers := c.RealState.Containers(app.Name)
              ctnrNum := len(containers)
              if ctnrNum != app.Instance {
                     // app status is created but container num not equal app Instance, return nil
                     return nil
              }
              passed := 0
 
              if len(app.Health) != 0 {
                     fs, err := fastsetting.GetFastsetting()
                     if err != nil {
                            return fmt.Errorf("get fastsetting error: %s", err.Error())
                     }
 
                     for _, container := range containers {
                            exist, err := fs.IsCTNRExecModeExist(container.ID)
                            if nil != err {
                                   glog.Errorf("IsCTNRExecModeExist: %s fail:%v", container.ID, err)
								   continue								   
                            }
                            if exist || c.healthCheckOneContainer(container) {
                                   passed++
                            }
                     }
              }
 
              if (ctnrNum == passed || len(app.Health) == 0) && app.Status != appStorage.AppStatusStartSuccess {
                     glog.Infof("health check success, name: %s, update status to %s", app.Name, appStorage.AppStatusStartSuccess)
                     err := c.UpdateAppStatus(app, appStorage.AppStatusStartSuccess, "")
                     if err != nil {
                            glog.Errorf("update app: %s status to: %s fail: %v", app.Name, appStorage.AppStatusStartSuccess, err)
                            return err
                     }
					 ctnrs := make([]realstate.Container, 0)
                     for _, ctnr := range containers {
                            ctnrs = append(ctnrs, *ctnr)
                     }
                     ME.NewEventReporter(
                            ME.AppStatusToStarted,
                            ME.AppStatusToStartedData{
                                   App:        *app,
                                   Containers: ctnrs,
                                   Birthday:   time.Now(),
                                   Message:    fmt.Sprintf("App %v Health Check Success  --> Check %v Containers", app.Name, passed),
                            })
                     // report to mwc
                     if app.Recovery != 0 && c.Config().RecoveryReport.Switch {
                            glog.Infof(`recovery from failure successfully, name: %s, http post to "%s"`, app.Name, c.Config().RecoveryReport.RestAddress)
							ctnrsJson, err := json.Marshal(containers)
                            if err != nil {
                                   glog.Error("json.Marshal fail: ", err)
                                   return err
                            }
                            reportJson := fmt.Sprintf(`{"name":"%s","containers":%s}`, app.Name, ctnrsJson)
                            _, err = httpPost("POST", c.Config().RecoveryReport.RestAddress, reportJson)
                            if err != nil {
                                   glog.Errorf("httpPost fail: %v", err)
                            }
 
                            // not in recovery
                            g.UpdateRecoveryByName(app.Name, false)
                     }
              }
       }
       err := c.afterHealthCheck(app)
       if err != nil {
              glog.Errorf("afterHealthCheck app: %s fail: %v", app.Name, err)
              return err
       }
       return nil
}

func (c *ContainerManager) afterHealthCheck(app *appStorage.App) error {
       // checkout container status
       if app.Status == appStorage.AppStatusStartSuccess {
              containers := c.RealState.Containers(app.Name)
              fastsettingInstance, err := fastsetting.GetFastsetting()
              if err != nil {
                     return err
              }
 
              recovery := false
              for _, container := range containers {
                     if strings.Contains(container.Status, realstate.ContainerStatusExited) {
                            isExist, err := fastsettingInstance.IsCTNRExecModeExist(container.ID)
                            if nil != err {
                                   glog.Errorf("IsCTNRExecModeExist: %s fail:%v", container.ID, err)
                                   continue
                            }
                            if isExist {
                                   continue
                            }
							// recovery from failure, drop old container first, it will created by update instance
                            recovery = true
                            glog.Warningf(`DropContainers, because its status is "%s"`, container.Status)
                            c.DropContainers(app.Name, 1, []*realstate.Container{container})
                     }
              }
              if recovery {
                     glog.Warningf(`app: %s, container status is "%s", need recovery from failure!`, app.Name, realstate.ContainerStatusExited)
                     // in recovery
                     g.UpdateRecoveryByName(app.Name, true)
                     c.UpdateAppStatus(app, appStorage.AppStatusPending, "container need recovery from failure")
              }
       }
       return nil
}

func httpPost(method, url, body string) (string, error) {
       client := &http.Client{}
       req, err := http.NewRequest(method, url, strings.NewReader(body))
       if err != nil {
              return "", err
       }
       req.Close = true
       req.Header.Set("Content-Type", "application/json")
 
       resp, err := client.Do(req)
       if err != nil {
              return "", err
       }
       defer resp.Body.Close()
 
       respbody, err := ioutil.ReadAll(resp.Body)
       if err != nil {
              return "", err
       }
       respData := string(respbody)
 
       //Success 2xx
       if http.StatusOK != resp.StatusCode {
              return respData, fmt.Errorf(`respone code: "%d", body: "%s"`, resp.StatusCode, respData)
       }
 
       return respData, nil
}
