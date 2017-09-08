package hbs
 
import (
       "errors"
       "strconv"
       "strings"
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
 
// NodeState contain RPC function calls.
type NodeState struct {
}
 
// Push save node infomation.
func (Node *NodeState) Push(req *node.NodeRequest, resp *node.NodeResponse) error {
       if req == nil {
              resp.Code = node.NilNodeRequestError
              return errors.New("node.NodeRequest can't be nil")
       }
       if g.Config().SignMsg.Switch {
              myaes, err := aes.New("")
              if err != nil {
                     glog.Error("create crypt fail:", err)
                     return err
              }
              src, err := myaes.Decrypt(req.Sign)
              if err != nil {
                     glog.Error("Decrypt fail:", err)
                     return err
              }
              tmpSign, err := strconv.Atoi(src)
              if err != nil {
                     glog.Error("string parse int fail:", err)
                     return err
              }
              sign := int64(tmpSign)
              if time.Now().Unix()-sign > g.Config().SignMsg.Interval && time.Now().Unix()-sign < -g.Config().SignMsg.Interval {
                     return errors.New("request refuse!")
              }
       }
 
       bCompatible := g.Config().MinionMinVersion.IsVersionIncompatible(req.Node.Version)
       if !bCompatible {
              resp.Code = node.IncompatibleVersionError
              glog.Errorf("minion ip:%s version:%s is not compatible with the scheduler required minium minion version:%s",
                     req.Node.IP, req.Node.Version, g.MINION_MINIMUM_VERSION)
       }
 
       updatetime := time.Now().Unix()
       req.Node.HeartBeatAt = updatetime
       req.Node.UpdateAt = updatetime
       getNode, exist := g.RealNodeState.GetNode(req.Node.IP)
       if !exist {
              glog.Warningf("add new node: %s", req.Node.IP)
              req.Node.CPUVirtUsage = req.Node.CPUUsage
              req.Node.MemVirtUsage = req.Node.MemUsage
              g.UpdateNode(&req.Node)
       } else {
              if req.Node.CPUVirtUsage > getNode.CPUVirtUsage || req.Node.MemVirtUsage > getNode.MemVirtUsage {
                     req.Node.CPUVirtUsage = getNode.CPUVirtUsage
                     req.Node.MemVirtUsage = getNode.MemVirtUsage
              } else {
                     req.Node.CPUVirtUsage = req.Node.CPUUsage
                     req.Node.MemVirtUsage = req.Node.MemUsage
              }
              g.UpdateNode (& req.Node)
       }
 
       if req.Containers == nil {
              return nil
       }
 
       for _, container: = range req.Containers {
              container.IP = req.IP
              container.UpdateAt = updatetime
              g.RealMinionState.UpdateContainer (container)
       }
 
       return nil
}
 
// NodeDown mark the node down.
func (Node *NodeState) NodeDown(ip string, resp *node.NodeResponse) error {
       if ip == "" {
              resp.Code = node.NilNodeRequestError
              return errors.New("ip can't be empty")
       }
 
       if err := g.DeleteNode(ip); err != nil {
              return err
       }
 
       g.DeleteContainerByIP(ip)
       return nil
}
 
func (Node *NodeState) Heartbeat(req *node.NodeHeartbeat, resp *node.NodeResponse) error {
       if req == nil {
              resp.Code = node.NilNodeRequestError
              return errors.New("node.NodeRequest can't be nil")
       }
       if g.Config().SignMsg.Switch {
              myaes, err := aes.New("")
              if err != nil {
                     glog.Error("create crypt fail:", err)
                     return err
              }
              src, err := myaes.Decrypt(req.Sign)
              if err != nil {
                     glog.Error("Decrypt fail:", err)
                     return err
              }
              tmpSign, err := strconv.Atoi(src)
              if err != nil {
                     glog.Error("string parse int fail:", err)
                     return err
              }
              sign := int64(tmpSign)
              if time.Now().Unix()-sign > g.Config().SignMsg.Interval || time.Now().Unix()-sign < -g.Config().SignMsg.Interval {
                     return errors.New("request refuse!")
              }
       }
 
       getNode, exist := g.RealNodeState.GetNode(req.NodeIP)
       if !exist {
              return nil
       }
       getNode.UpdateAt = time.Now().Unix()
       getNode.Fails = req.Fails
       g.UpdateNode(getNode)
       return nil
}
 
func (Node *NodeState) AppTaskResult(req *createstate.AppTaskCreateCtnrResult, resp *createstate.RPCCommonRespone) error {
       dockStatus := realstate.DockerStatusNew(g.RedisConnPool)
       result := realstate.Result{}
       state, err := dockStatus.GetCreatingStatus(req.AppName)
       if err != nil {
              result.CreateUnixTime = time.Now().Unix()
              glog.Error("GetCreatingStatus failed. Error: ", err)
       } else {
              result.CreateUnixTime = state.CreateUnixTime
       }
 
       for _, err := range req.Fail {
              glog.Infof("run container fail, name: %s, err: %s, NodeIp:%s", req.AppName, err, req.NodeIp)
              ME.NewEventReporter(ME.ExecResult, ME.ExecResultData{
                     Reason:   "AppTaskResult run container failed",
                     AppName:  req.AppName,
                     Error:    err,
                     Birthday: time.Now(),
              })
              result.Fail++
       }
 
       for _, ctnr := range req.Success {
              glog.Infof("run container success, Appname: %s, ctrnid: %s, ctrnName:%s, nodeIP:%s", ctnr.AppName, ctnr.ID, ctnr.ContainerName, req.NodeIp)
              ctnr.UpdateAt = time.Now().Unix()
              g.RealState.UpdateContainer(ctnr)
              result.Success++
       }
       if result.Fail != 0 {
              g.UpdateAppStatusByName(req.AppName, app.AppStatusPending, strings.Join(req.Fail, ";"))
       }
       err = dockStatus.DockerStatusUpdateResult(req.AppName, result)
       if err != nil {
              glog.Warningf("App %s CreatingState.DockerStatusUpdateResult fail: %v", req.AppName, err)
       }
       return nil
}
