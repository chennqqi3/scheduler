package sync
 
//the same as lrp_type
import (
       "errors"
       "fmt"
       "strings"
       "time"
 
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       appStorage "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/common/util/uuid"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/scheduler/executor"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm"
)
 
//OOT container
type ContainerOOTManager struct {
       SyncController
}

func NewContainerOOTManager(
       realState g.ContainerRealState,
       creatingState realstate.DockerRunStatusProvider,
       nodes g.MinionStorageDriver,
       config ConfigFunc,
       algo *algorithm.Config,
       updateAppStatusByName UpdateAppStatusByNameFunc,
) SchedulerProvider {
       return &ContainerOOTManager{
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
 
func (c *ContainerOOTManager) StatusCompare(app *appStorage.App) error {
       if app == nil {
              return errors.New("app is nil")
       }
	   //Status Arrangement
       switch app.Status {
       case appStorage.AppStatusPending:
              finish := func(status *realstate.DockerStatusStore) error {
                     if status.Result.Success == status.Count {
                            c.UpdateAppStatus(app, appStorage.AppStatusCreateSuccess, "")
                     } else {
                            c.UpdateAppStatus(app, appStorage.AppStatusCreateContainerFail, "app status create container fail")
                     }
 
                     for _, node := range status.Node {
                            virNode, exist := g.RealNodeState.GetNode(node.IP)
                            if !exist {
                                   glog.Warningf("virNode is not exist, IP: %s", node.IP)
                                   continue
                            }
							virNode.CPUVirtUsage = virNode.CPUVirtUsage   node.Cpu
                            virNode.MemVirtUsage = virNode.MemVirtUsage   node.Memory
                            g.RealNodeState.UpdateNode(virNode)
                     }
                     return nil
              }
              err := c.CreatingStatus(app.Name, app.Instance, 0, func() error { return nil }, finish)
              if err != nil {
                     return err
              }
 
       case appStorage.AppStatusCreateSuccess:
              fallthrough
       case appStorage.AppStatusStartSuccess:
              ctnrs := c.RealState.Containers(app.Name)
              count := 0
              for _, ctnr := range ctnrs {
                     if strings.Contains(ctnr.Status, realstate.ContainerStatusExited) || strings.Contains(ctnr.Status, realstate.ContainerStatusDown) {
                            count  
                     }
              }
			  if count != 0 && count == app.Instance {
                     glog.Infof("app(OOT) is At the end of the run, name: %s", app.Name)
                     c.UpdateAppStatus(app, appStorage.AppStatusCMDExit, "app status CMD exit")
              }
       }
       return nil
}
 
func (c *ContainerOOTManager) CreateContainers(app *appStorage.App) error {
       err := c.createSomeContainers(app, app.Instance)
       if err != nil {
              return err
       }
       return nil
}
 
func (c *ContainerOOTManager) createSomeContainers(app *appStorage.App, num int) error {
       callBack := func() error {
              if num == 0 {
                     return nil
              }
			  glog.Infof("CreateContainers, name: %s, num: %d", app.Name, num)
              ipCount, err := c.Algo.Algorithm.Schedule(app, num)
              if err != nil {
                     glog.Error("Algorithm.Schedule error: ", err)
                     c.UpdateAppStatus(app, appStorage.AppStatusNoSelectNode, err.Error())
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
					 virNode.CPUVirtUsage = virNode.CPUVirtUsage   node.Cpu
                     virNode.MemVirtUsage = virNode.MemVirtUsage   node.Memory
                     g.RealNodeState.UpdateNode(virNode)
              }
              return nil
       }
       err := c.CreatingStatus(app.Name, app.Instance, num, callBack, finish)
       return err
}
 
func (c *ContainerOOTManager) UpdateContainers(app *appStorage.App) error {
       return nil
}

func (c *ContainerOOTManager) DropContainers(appName string, appInstance int, containers []*realstate.Container) error {
       callBack := func() error {
              glog.Warningf("DropContainers, name: %s, num: %d", appName, len(containers))
              err := c.CreatingState.DockerStatusUpdate(
                     appName, len(containers),
                     nil, StatusDeleteTimeOut, StatusDeleteTimeAgeing)
              if err != nil {
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

func (c *ContainerOOTManager) runContainers(app *appStorage.App, ipCount map[string]int) {
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
                     deployNode := &node.DeployInfo{
                            AgentIP:  selectNodeIP,
                            Ctnrname: name(),
                     }
                     nodeAppDeployInfo.DeployInfos = append(nodeAppDeployInfo.DeployInfos, deployNode)
              }
              go executor.GetDockerManager().NodeTaskCreateCtnr(selectNodeIP, []*node.NodeAppDeployInfo{nodeAppDeployInfo})
       }
       return
}

func (c *ContainerOOTManager) RequireResource(app *appStorage.App) error {
       err := c.UpdateAppStatus(app, appStorage.AppStatusPending, "")
       if err != nil {
              return err
       }
       return nil
}
 
func (c *ContainerOOTManager) healthCheckOneContainer(container *realstate.Container) (ok bool) {
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
func (c *ContainerOOTManager) HealthCheck(app *appStorage.App) error {
       if !c.RealState.RealAppExists(app.Name) {
              return fmt.Errorf("real state do not exist for app: %s", app.Name)
       }
 
       if app.Status == appStorage.AppStatusCreateSuccess {
              var passed int
 
              containers := c.RealState.Containers(app.Name)
              if len(app.Health) != 0 {
                     for _, container := range containers {
                            if c.healthCheckOneContainer(container) {
                                   passed++
                            }
                     }
              }
 
              if len(containers) == passed || len(app.Health) == 0 {
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
              }
       }
       return nil
}
