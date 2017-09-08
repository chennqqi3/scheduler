package executor
 
import (
       "fmt"
       "net/rpc"
       "strings"
       "sync"
       "time"
 
       "github-beta.huawei.com/hipaas/common/crypto/aes"
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/createstate"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/scheduler/g"
)
 
const (
       Create  = "Create"
       Drop    = "Drop"
       Stop    = "Stop"
       Restart = "Restart"
)
 
type DockerInstance struct {
       App        *app.App
       DeployInfo *node.DeployInfo
}
 
type DockerRuntimeProvider interface {
       DockerExecAsync(exec string, arg interface{}, beforeExec func() error, afterExec func(error))
       GetDockerLogs(appName string, ctnrID string) ([]*createstate.DockerLogs, error)
       NodeTaskCreateCtnr(nodeIP string, appsDeployInfos []*node.NodeAppDeployInfo) error
}
 
type dockerManager struct {
       sem chan struct{}
}
 
func newDockerManager() *dockerManager {
       return &dockerManager{
              sem: make(chan struct{}, g.Config().MaxDockerExecCount),
       }
}
 
// Singleton pattern
var dockerManagerInterface DockerRuntimeProvider
var once sync.Once
 
func GetDockerManager() DockerRuntimeProvider {
       once.Do(func() {
              dockerManagerInterface = newDockerManager()
       })
       return dockerManagerInterface
}
 
func (this *dockerManager) DockerExecAsync(exec string, arg interface{}, beforeExec func() error, afterExec func(error)) {
       switch exec {
       case Create:
              if value, ok := arg.(*DockerInstance); ok {
                     exec := func() error {
                            err := this.dockerCreate(value.App, value.DeployInfo)
                            return err
                     }
                     this.dockerExecAsync(exec, beforeExec, afterExec)
              } else {
                     glog.Errorf("%s docker exec fail: type Error!", exec)
              }
       case Drop:
              if value, ok := arg.(*realstate.Container); ok {
                     exec := func() error {
                            err := this.dockerDrop(value)
                            return err
                     }
                     this.dockerExecAsync(exec, beforeExec, afterExec)
              } else {
                     glog.Errorf("%s docker exec fail: type Error!", exec)
              }
       case Stop:
              if value, ok := arg.(*realstate.Container); ok {
                     exec := func() error {
                            err := this.dockerStop(value)
                            return err
                     }
                     this.dockerExecAsync(exec, beforeExec, afterExec)
              } else {
                     glog.Errorf("%s docker exec fail: type Error!", exec)
              }
       case Restart:
              if value, ok := arg.(*realstate.Container); ok {
                     exec := func() error {
                            err := this.dockerRestart(value)
                            return err
                     }
                     this.dockerExecAsync(exec, beforeExec, afterExec)
              } else {
                     glog.Errorf("%s docker exec fail: type Error!", exec)
              }
       default:
              glog.Errorf("undefined docker exec type: %s", exec)
       }
}
 
func (this *dockerManager) afterDockerDrop(appName string, container *realstate.Container) {
       g.RealState.DeleteContainer(appName, container)
}
 
// DockerRun run a docker container on the client[ip]
func (this *dockerManager) dockerExecAsync(exec func() error, beforeExec func() error, afterExec func(error)) {
       this.acquire()
       go func() {
              defer this.release()
              if err := beforeExec(); err != nil {
                     return
              }
              err := exec()
              afterExec(err)
       }()
}
 
func (this *dockerManager) acquire() { this.sem <- struct{}{} }
func (this *dockerManager) release() { <-this.sem }
 
// DockerRun run a docker container on the client[ip]
func (this *dockerManager) dockerCreate(app *app.App, deployInfo *node.DeployInfo) error {
       glog.Infof("create container. name: %s, node ip: %s", app.Name, deployInfo.AgentIP)
 
       agentRPCAddr := fmt.Sprintf("%s:%d", deployInfo.AgentIP, g.Config().AgentRPCPort)
       client, err := rpc.Dial("tcp", agentRPCAddr)
       if err != nil {
              return err
       }
       defer client.Close()
 
       req, err := ParaRunContainerOption(app, deployInfo)
       if err != nil {
              glog.Error("para runContainerOption fail: ", err)
              return err
       }
 
       if g.Config().SignMsg.Switch {
              req.Sign, err = encrypt()
              if err != nil {
                     glog.Error("encrypt sign string fail: ", err)
                     return err
              }
       }
       var resp createstate.RunContainerResponse
       // now := time.Now()
       // glog.Debugf("begin to run docker run: %v", time.Now())
       err = client.Call("Execute.NewDockRun", req, &resp)
       if err != nil {
              glog.Errorf("Create app fail, ip: %s, err: %v", deployInfo.AgentIP, err)
              return err
       }
       // glog.Debugf("end to run docker run: %v, spend %v", time.Now(), time.Now().Sub(now))
       glog.Infof("run container success, name: %s, id: %s", resp.Container.AppName, resp.Container.ID)
       resp.Container.UpdateAt = time.Now().Unix()
       g.RealState.UpdateContainer(resp.Container)
       return nil
}
 
func (this *dockerManager) dockerDrop(c *realstate.Container) error {
       glog.Info("drop container:", c)
 
       agentRPCAddr := fmt.Sprintf("%s:%d", c.IP, g.Config().AgentRPCPort)
       client, err := rpc.Dial("tcp", agentRPCAddr)
       if err != nil {
              glog.Error("dialing fail: ", err)
              return err
       }
       defer client.Close()
 
       sign, err := encrypt()
       if err != nil {
              glog.Error("encrypt fail:", err)
              return err
       }
       c.Sign = sign
 
       var resp createstate.CommonContainerExecResponse
       err = client.Call("Execute.DockDel", c, &resp)
       if err != nil {
              if strings.Contains(err.Error(), "No such container") {
                     glog.Warning("no such container, delete real state by force! err: ", err)
                     this.afterDockerDrop(c.AppName, c)
                     return err
              }
 
              glog.Errorf("docker.RemoveContainer fail, ip: %s, err: %v", c.IP, err)
              return err
       }
       glog.Infof("drop container success, name: %s, id: %s", c.AppName, c.ID)
       this.afterDockerDrop(c.AppName, c)
       return nil
}
 
func (this *dockerManager) dockerStop(ctnr *realstate.Container) error {
       glog.Info("stop container, id : ", ctnr.ID)
 
       agentRPCAddr := fmt.Sprintf("%s:%d", ctnr.IP, g.Config().AgentRPCPort)
       client, err := rpc.Dial("tcp", agentRPCAddr)
       if err != nil {
              glog.Error("dialing fail: ", err)
              return err
       }
       defer client.Close()
 
       sign, err := encrypt()
       if err != nil {
              glog.Error("encrypt fail:", err)
              return err
       }
       ctnr.Sign = sign
 
       var resp createstate.RunContainerResponse
       err = client.Call("Execute.DockerStop", ctnr, &resp)
       if err != nil {
              glog.Errorf("docker.StopContainer fail, ip: %s, err: %v", ctnr.IP, err)
              return err
       }
       glog.Infof("stop container success, name: %s, id: %s", ctnr.AppName, ctnr.ID)
       resp.Container.UpdateAt = time.Now().Unix()
       g.RealState.UpdateContainer(resp.Container)
       return nil
}
 
func (this *dockerManager) dockerRestart(ctnr *realstate.Container) error {
       glog.Warning("restart container, id : ", ctnr.ID)
 
       agentRPCAddr := fmt.Sprintf("%s:%d", ctnr.IP, g.Config().AgentRPCPort)
       client, err := rpc.Dial("tcp", agentRPCAddr)
       if err != nil {
              glog.Error("dialing fail: ", err)
              return err
       }
       defer client.Close()
 
       sign, err := encrypt()
       if err != nil {
              glog.Error("encrypt fail:", err)
              return err
       }
       ctnr.Sign = sign
 
       var resp createstate.RunContainerResponse
       err = client.Call("Execute.DockerRestart", ctnr, &resp)
       if err != nil {
              glog.Errorf("docker.RestartContainer fail, ip: %s, err: %v", ctnr.IP, err)
              return err
       }
       glog.Infof("restart container success, name: %s, id: %s", ctnr.AppName, ctnr.ID)
       resp.Container.UpdateAt = time.Now().Unix()
       g.RealState.UpdateContainer(resp.Container)
       return nil
}
 
func (this *dockerManager) GetDockerLogs(appName string, ctnrID string) ([]*createstate.DockerLogs, error) {
       mapCtnrs := g.RealState.GetContainersMap(appName)
       if len(mapCtnrs) == 0 {
              return nil, fmt.Errorf("Can't find any container by this app name: %s", appName)
       }
 
       totalDockerLogs := []*createstate.DockerLogs{}
       if ctnrID != "" {
              if value, exist := mapCtnrs[ctnrID]; exist {
                     logs, err := this.dockerLogs(value)
                     if err != nil {
                            glog.Errorf("dockerLogs fail, name: %s, id: %s err: %v", appName, ctnrID, err)
                            return nil, err
                     }
                     dockerLogs := &createstate.DockerLogs{AppName: appName, CtnrID: ctnrID, Logs: logs}
                     totalDockerLogs = append(totalDockerLogs, dockerLogs)
                     return totalDockerLogs, err
              }
              return nil, fmt.Errorf("ContainerID(%s) isn't matched appName(%s)", ctnrID, appName)
       }
       for _, ctnr := range mapCtnrs {
              logs, err := this.dockerLogs(ctnr)
              if err != nil {
                     glog.Errorf("dockerLogs fail, name: %s, id: %s err: %v", appName, ctnr.ID, err)
                     return nil, err
              }
              dockerLogs := &createstate.DockerLogs{AppName: appName, CtnrID: ctnr.ID, Logs: logs}
              totalDockerLogs = append(totalDockerLogs, dockerLogs)
       }
       return totalDockerLogs, nil
}
 
func (this *dockerManager) dockerLogs(ctnr *realstate.Container) (*string, error) {
       glog.Infof("get container logs, id: %s", ctnr.ID)
 
       agentRPCAddr := fmt.Sprintf("%s:%d", ctnr.IP, g.Config().AgentRPCPort)
       client, err := rpc.Dial("tcp", agentRPCAddr)
       if err != nil {
              glog.Error("dialing fail: ", err)
              return nil, err
       }
       defer client.Close()
 
       sign, err := encrypt()
       if err != nil {
              glog.Error("encrypt fail:", err)
              return nil, err
       }
       ctnr.Sign = sign
 
       response := new(string)
       err = client.Call("Execute.DockerLogs", ctnr, &response)
       if err != nil {
              glog.Errorf("docker.Logs fail, ip: %s, err: %v", ctnr.IP, err)
              return nil, err
       }
       glog.Infof("get container logs success, name: %s, id: %s", ctnr.AppName, ctnr.ID)
       return response, nil
}
 
func (this *dockerManager) NodeTaskCreateCtnr(nodeIP string, appsDeployInfos []*node.NodeAppDeployInfo) (err error) {
       defer func() {
              if nil != err {
                     for _, appDeployInfos := range appsDeployInfos {
                            for _, deployInfo := range appDeployInfos.DeployInfos {
                                   ME.NewEventReporter(ME.ExecResult, ME.ExecResultData{
                                          Reason:   "NodeTaskCreateCtnr failed",
                                          AppName:  appDeployInfos.App.Name,
                                          NodeIP:   nodeIP,
                                          CtnrName: deployInfo.Ctnrname,
                                          Error:    err.Error(),
                                          Birthday: time.Now(),
                                          Message:  fmt.Sprintf("App %v Create container fail  --> node: %v, containerName: %v, err: %v", appDeployInfos.App.Name, nodeIP, deployInfo.Ctnrname, err),
                                   })
                            }
                     }
              }
       }()
 
       nodeTask := &createstate.NodeTaskCreateCtnr{}
       if g.Config().SignMsg.Switch {
       //     var err error
              nodeTask.Sign, err = encrypt()
              if err != nil {
                     glog.Error("encrypt sign string fail: ", err)
                     return err
              }
       }
       for _, appDeployInfos := range appsDeployInfos {
              appTask := &createstate.AppTaskCreateCtnr{AppName: appDeployInfos.App.Name}
              for _, deployInfo := range appDeployInfos.DeployInfos {
                     glog.Infof("run node task. node ip: %s, name: %s, container: %s", deployInfo.AgentIP, appDeployInfos.App.Name, deployInfo.Ctnrname)
                     opt, err := ParaRunContainerOption(appDeployInfos.App, deployInfo)
                     if err != nil {
                            glog.Error("para runContainerOption fail: ", err)
                            return err
                     }
                     opt.Sign = nodeTask.Sign
                     appTask.Opts = append(appTask.Opts, opt)
 
                     ME.NewEventReporter(ME.CreateOneContainerForAnyReason,
                            ME.CreateOneContainerForAnyReasonData{
                                   AppName:  appDeployInfos.App.Name,
                                   NodeIP:   nodeIP,
                                   CtnrName: deployInfo.Ctnrname,
                                   Birthday: time.Now(),
                                   Message:  fmt.Sprintf("App %v Create one container  --> node: %v, containerName: %v", appDeployInfos.App.Name, nodeIP, deployInfo.Ctnrname),
                            })
              }
              nodeTask.AppTasks = append(nodeTask.AppTasks, appTask)
       }
 
       agentRPCAddr := fmt.Sprintf("%s:%d", nodeIP, g.Config().AgentRPCPort)
       client, err := rpc.Dial("tcp", agentRPCAddr)
       if err != nil {
              return err
       }
       defer client.Close()
 
       req := nodeTask
       resp := &createstate.RPCCommonRespone{}
       err = client.Call("Execute.NodeTaskCreateCtnr", req, resp)
       if err != nil {
              glog.Errorf("Create app fail, ip: %s, err: %v", nodeIP, err)
              return err
       }
       return nil
}
 
//use Encrypt Interval
func encrypt() (string, error) {
       src := fmt.Sprintf("%d", time.Now().Unix())
       myaes, err := aes.New("")
       if err != nil {
              glog.Error("create Crypto fail:", err)
              return "", err
       }
       res, err := myaes.Encrypt(src)
       if err != nil {
              glog.Error("Encrypt fail:", err)
              return "", err
       } else {
              return res, nil
       }
}
