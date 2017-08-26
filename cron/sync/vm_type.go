package sync
 
import (
       "errors"
       "fmt"
       "strings"
       "sync"
       "time"
 
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       appStorage "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/network"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/common/util/ping"
       "github-beta.huawei.com/hipaas/common/util/uuid"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/netservice/client"
       "github-beta.huawei.com/hipaas/scheduler/executor"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm"
)

//vm container
type VMManager struct {
       SyncController
 
       UpdateHostnameStatusFunc UpdateHostnameStatusFunc
       GetRegionByMinionIPFunc  GetRegionByMinionIPFunc
       sdkOfNetservice          *netservice.Client
       requireResourceTries     map[int64]int
       requireResourceTriesLock sync.Mutex
}
 
func NewVMManager(
       realState g.ContainerRealState,
       creatingState realstate.DockerRunStatusProvider,
       nodes g.MinionStorageDriver,
       config ConfigFunc,
       algo *algorithm.Config,
       updateAppStatusByName UpdateAppStatusByNameFunc,
       updateHostnameStatus UpdateHostnameStatusFunc,
       getRegionByMinionIP GetRegionByMinionIPFunc,
 
) SchedulerProvider {
       return &VMManager{
              SyncController: SyncController{
                     RealState:     realState,
					 CreatingState: creatingState,
                     Nodes:         nodes,
                     Config:        config,
                     Algo:          algo,
                     UpdateAppStatusByNameFunc: updateAppStatusByName,
              },
 
              UpdateHostnameStatusFunc: updateHostnameStatus,
              GetRegionByMinionIPFunc:  getRegionByMinionIP,
              sdkOfNetservice:          g.NetClient(),
              requireResourceTries:     make(map[int64]int),
       }
}
 
func (c *VMManager) StatusCompare(app *appStorage.App) error {
       //Status Arrangement
       if app.Status == appStorage.AppStatusPending {
              ctnrUpCount := 0
              containers := c.RealState.Containers(app.Name)
              for _, container := range containers {
                     c.UpdateHostnameStatusFunc(app, container.Hostname, appStorage.AppStatusCreateSuccess)
                     ctnrUpCount  
              }
			  if ctnrUpCount == len(app.Hostnames) {
                     glog.Infof("status compare success, name: %s, update status to %s", app.Name, appStorage.AppStatusCreateSuccess)
                     c.UpdateAppStatus(app, appStorage.AppStatusCreateSuccess, "")
              } else {
                     errHaveOccurred := false
                     for _, hostname := range app.Hostnames {
                            switch hostname.Status {
                            case appStorage.AppStatusPending:
                            case appStorage.AppStatusCreateSuccess:
                            default:
                                   errHaveOccurred = true
                            }
                     }
 
                     if errHaveOccurred {
                            glog.Errorf("VM type app create fail, name: %s", app.Name)
							c.UpdateAppStatus(app, appStorage.AppStatusCreateContainerFail, "VM type app create fail, for details, see hostname logs.")
 
                            // IP RollBack
                            for _, hostname := range app.Hostnames {
                                   c.releaseIPRequest(hostname.Hostname, hostname.Subdomain, app.Region)
                            }
                     }
              }
       }
       //creating status compare
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
       err := c.CreatingStatus(app.Name, app.Instance, 0, func() error { return nil }, finish)
       return err
}
 
func (c *VMManager) CreateContainers(app *appStorage.App) error {
       hostnames, err := app.GetHostname()
       if err != nil {
              return err
       }
       err = c.runContainersByHostname(app, hostnames)
 
       ME.NewEventReporter(ME.StartToCreateApp,
              ME.StartToCreateAppData{
                     App:      *app,
                     Birthday: time.Now(),
              })
 
       return err
}

func (c *VMManager) UpdateContainers(app *appStorage.App) error {
       if app.Status == appStorage.AppStatusCreateContainerFail {
              return nil
       }
 
       hostnames, err := app.GetHostname()
       if err != nil {
              return err
       }
       hostLen := len(hostnames)
       containers := c.GetRealStateCTNRs(app.Name)
 
       if len(containers) != hostLen {
              glog.Infof("update %s from instance %d to %d", app.Name, len(containers), hostLen)
       }
 
       delContainerSlice, hostnameDeployed, err := c.RealState.StateCompare(hostnames, app.Name)
       if err != nil {
              glog.Errorf("update containers, statecompare fail: %v", err)
              return err
       }
	   if len(delContainerSlice) > 0 {
              c.DropContainers(app.Name, app.Instance, delContainerSlice)
       }
 
       deployHostname := []string{}
       for hostname, isContainerExist := range hostnameDeployed {
              if isContainerExist {
                     continue
              }
              deployHostname = append(deployHostname, hostname)
       }
 
       if len(deployHostname) == 0 {
              return nil
       }
 
       err = c.runContainersByHostname(app, deployHostname)
       return err
}

func (c *VMManager) DropContainers(appName string, appInstance int, containers []*realstate.Container) error {
       //creating status compare
       callBack := func() error {
              glog.Warningf("DropContainers, name: %s, num: %d", appName, len(containers))
              if len(containers) == 0 {
                     return nil
              }
              err := c.CreatingState.DockerStatusUpdate(
                     appName, len(containers),
                     nil, StatusDeleteTimeOut, StatusDeleteTimeAgeing)
              if err != nil {
                     return err
              }
              for _, container := range containers {
                     err := c.releaseIP(container)
                     if err != nil {
                            continue
                     }
 
                     c.DropOneContainer(container)
              }
              return nil
       }
	   finish := func(status *realstate.DockerStatusStore) error {
              return nil
       }
       err := c.CreatingStatus(appName, 0, len(containers), callBack, finish)
       return err
}
 
func (c *VMManager) runContainersByHostname(app *appStorage.App, hostnames []string) error {
       hostLen := len(hostnames)
       //creating status compare
       callBack := func() error {
              if len(hostnames) == 0 {
                     glog.Warning("app hostname is null: ", app)
                     return nil
              }
              glog.Info("runContainersByHostname: ", app.Name)
              ipCount, err := c.Algo.Algorithm.Schedule(app, len(hostnames))
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
              glog.Info("chooseNode is: ", ipCount)
 
              for selectNodeIP, count := range ipCount {
                     nodeAppDeployInfo := &node.NodeAppDeployInfo{App: app}
					 for k := 0; k < count; k++ {
                            hostname := hostnames[hostLen-1]
                            hostLen--
                            network, err := c.findNetworkByHostname(hostname)
                            if err != nil {
                                   glog.Errorf("hostname: %s get availe ip address fail: %v", hostname, err)
                                   continue
                            }
 
                            name := func() string {
                                   if id := uuid.GetGuid(); 13 <= len(id) {
                                          return id[0:13] + "_" + app.GetSubHostname(hostname)
                                   } else {
                                          return id + "_" + app.GetSubHostname(hostname)
                                   }
                            }
							deployNode := &node.DeployInfo{
                                   Hostname:    *network.Hostname,
                                   Ctnrname:    name(),
                                   Region:      network.Region,
                                   ContainerIP: network.Ip,
                                   Gateway:     network.Gateway,
                                   Mask:        network.Mask,
                                   AgentIP:     selectNodeIP,
                            }
 
                            nodeAppDeployInfo.DeployInfos = append(nodeAppDeployInfo.DeployInfos, deployNode)
                     }
 
                     go executor.GetDockerManager().NodeTaskCreateCtnr(selectNodeIP, []*node.NodeAppDeployInfo{nodeAppDeployInfo})
              }
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
       err := c.CreatingStatus(app.Name, app.Instance, hostLen, callBack, finish)
       return err
}
 
func (c *VMManager) getAvailableIpAddress(hostname string, subdomain string, region string) (*network.Network, error) {
       apply := netservice.Applicants{
              Hostname:  hostname,
              Subdomain: subdomain,
              Region:    region,
       }

networkApply, err := c.sdkOfNetservice.ApplyNetwork(apply)
       if err != nil {
              return nil, err
       }
       glog.Infof("apply network is : %v ", networkApply)
       return &networkApply, nil
}
 
func (c *VMManager) releaseIP(container *realstate.Container) error {
       subdomain := container.Hostname[1+strings.IndexRune(container.Hostname, '.'):]
       hostname := container.Hostname[:strings.IndexRune(container.Hostname, '.')]
 
       err := c.releaseIPRequest(hostname, subdomain, container.Region)
       if nil != err {
              glog.Error("release container ip resource failed. container:", container)
              return err
       }
       return nil
}

func (c *VMManager) releaseIPRequest(hostname string, subdomain string, region string) error {
       glog.Infof("release container ip resource, hostname: %s, subdomain: %s, region: %s",
              hostname, subdomain, region)
 
       apply := netservice.Applicants{
              Hostname:  hostname,
              Subdomain: subdomain,
              Region:    region,
       }
 
       _, err := c.sdkOfNetservice.ReleaseNetwork(apply)
       if err != nil {
              glog.Errorf("release network resource fail, hostname: %s, err: %v", hostname, err)
              return err
       }
       return nil
}

func (c *VMManager) findNetworkByHostname(hostname string) (network.Network, error) {
       return c.sdkOfNetservice.FindNetworkByHostname(hostname)
}
 
func (c *VMManager) RequireResource(app *appStorage.App) error {
       applySuccess := 0
       errMessages := []string{}
 
       for _, hostname := range app.Hostnames {
              if hostname.Status == appStorage.AppStatusPending {
                     applySuccess  
                     continue
              }
 
              foundNetwork, err := c.findNetworkByHostname(hostname.Hostname   "."   hostname.Subdomain)
              if err != nil {
                     glog.Infof("findNetworkByHostname failed, hostname: %s, err: %v", hostname.Hostname, err)
					 errMessage := fmt.Sprintf("hostname: %v, err: %v", hostname, err)
                     errMessages = append(errMessages, errMessage)
              }
 
              if foundNetwork.Status == network.SUCCESS {
                     //if container ip is used, return error
                     pass, err := ping.Ping(foundNetwork.Ip)
                     if err != nil {
                            glog.Warningf("dial container ip: %s error: %v", foundNetwork.Ip, err)
                            errMessage := fmt.Sprintf("hostname: %v, err: %v", hostname, err)
                            errMessages = append(errMessages, errMessage)
                            continue
                     }
					 if !pass {
                            glog.Infof("useless ip: %s, use for creating new VM, name: %s, hostname: %s", foundNetwork.Ip, app.Name, hostname)
                            c.UpdateHostnameStatusFunc(app, hostname.Hostname+"."+hostname.Subdomain, appStorage.AppStatusPending)
                            applySuccess++
                     } else {
                            glog.Infof("ip has already been used: %s", foundNetwork.Ip)
                            errMessage := fmt.Sprintf("hostname: %v, err: %v", hostname, errors.New("ip has been used"))
                            errMessages = append(errMessages, errMessage)
 
                            // IP RollBack
                            c.releaseIPRequest(hostname.Hostname, hostname.Subdomain, app.Region)
                     }
                     continue
              }
			  if foundNetwork.Status != network.REQUIRING {
                     glog.Info("require ip resource for: ", hostname)
                     _, err = c.getAvailableIpAddress(hostname.Hostname, hostname.Subdomain, app.Region)
                     if err != nil {
                            glog.Errorf("require ip resource fail, hostname: %s, err: %v", hostname.Hostname, err)
                            errMessage := fmt.Sprintf("hostname: %v, err: %v", hostname, err)
                            errMessages = append(errMessages, errMessage)
                     }
              }
       }
 
       increaseAndGetTries := func() int {
              c.requireResourceTriesLock.Lock()
              defer c.requireResourceTriesLock.Unlock()
              c.requireResourceTries[app.Id]++
              tries := c.requireResourceTries[app.Id]
              return tries
       }
	   
	   deleteTries := func() {
              c.requireResourceTriesLock.Lock()
              defer c.requireResourceTriesLock.Unlock()
              delete(c.requireResourceTries, app.Id)
       }
 
       if applySuccess == len(app.Hostnames) {
              deleteTries()
              c.UpdateAppStatus(app, appStorage.AppStatusPending, "")
              return nil
       }
 
       if increaseAndGetTries() >= 10 {
              deleteTries()
              err := fmt.Errorf(`app IP resource require fail, name: %s, details: "%s"`, app.Name, strings.Join(errMessages, ";"))
              c.UpdateAppStatus(app, appStorage.AppStatusIPResourceRequireFail, err.Error())
			  // IP Rollback
              glog.Warningf("Requiring IP resource for app %s failed, rolling back", app.Name)
              for _, hostname := range app.Hostnames {
                     c.releaseIPRequest(hostname.Hostname, hostname.Subdomain, app.Region)
              }
              return err
       }
       return nil
}
 
func (c *VMManager) healthCheckOneContainer(container *realstate.Container) (ok bool) {
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
              glog.Warningf("health check ctnrid:%s ;appname:%s ; nodeip:%s; ctnrip:%s ; status unknown. ",
                     container.ID, container.AppName, container.IP, container.ContainerIP)
              return false
	   case realstate.CTNR_HEALTH_DEAD:
              glog.Errorf("health check container failed. ctnrid:%s ;appname:%s ; nodeip:%s; ctnrip:%s ",
                     container.ID, container.AppName, container.IP, container.ContainerIP)
              return false
       default:
              glog.Errorf(" health check mode:%v does not exist. ctnrid:%s, ip:%s",
                     container.HealthMode, container.ID, container.IP)
              return false
       }
 
}

func (c *VMManager) HealthCheck(app *appStorage.App) error {
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
                                   err := c.UpdateHostnameStatusFunc(app, container.Hostname, appStorage.AppStatusStartSuccess)
								   if err != nil {
                                          glog.Errorf("update hostname: %s status to: %s fail: %v",
                                                 container.Hostname,
                                                 appStorage.AppStatusStartSuccess, err)
                                   }
                            }
                     }
              }
 
              if len(app.Hostnames) == passed || len(app.Health) == 0 {
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
