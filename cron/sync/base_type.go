package sync
 
import (
       "encoding/json"
       "fmt"
       "time"
 
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/scheduler/executor"
       "github-beta.huawei.com/hipaas/scheduler/fastsetting"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm"
)
 
type ConfigFunc func() g.GlobalConfig
 
// type UpdateAppStatusFunc func(appId int, status string, log string) error
type UpdateAppStatusByNameFunc func(appName string, status string, log string) error
type CreatingStatusCallBackFunc func() error
type CreatingFinishCallBackFunc func(status *realstate.DockerStatusStore) error
type UpdateHostnameStatusFunc func(oneApp *app.App, hostname string, status string) error
type GetRegionByMinionIPFunc func(ip string) (string, error)
 
const (
       StatusDeleteTimeOut    = 600
       StatusDeleteTimeAgeing = StatusDeleteTimeOut * 2
       StatusCreateTimeOut    = 600
       StatusCreateTimeAgeing = StatusCreateTimeOut * 2
)
 
type SyncController struct {
       RealState                 g.ContainerRealState
       CreatingState             realstate.DockerRunStatusProvider
       Nodes                     g.MinionStorageDriver
       Config                    ConfigFunc
       Algo                      *algorithm.Config
       UpdateAppStatusByNameFunc UpdateAppStatusByNameFunc
}
type SchedulerProvider interface {
       StatusCompare(app *app.App) error
       CreateContainers(app *app.App) error
       UpdateContainers(app *app.App) error
       DropContainers(appName string, appInstanceCnt int, containers []*realstate.Container) error
       RequireResource(app *app.App) error
       HealthCheck(app *app.App) error
       // CheckStable(appName string, containers []*realstate.Container, before int64) error
}
 
func (c *SyncController) CreatingStatus(appName string, appInstanceCnt int, num int, createCall CreatingStatusCallBackFunc, finishCall CreatingFinishCallBackFunc) error {
       //Get the state has been created.
       status, err := c.CreatingState.DockerStatusSearch(appName)
       if err != nil {
              glog.Error("[ERROR] DockerStatusSearch error: ", err)
              return err
       }
	   switch status {
       case realstate.DockerStatusNotExist:
              if num == 0 {
                     return nil
              }
              err = createCall()
              if err != nil {
                     return err
              }
       case realstate.DockerStatusRunning:
              state, err := c.CreatingState.GetCreatingStatus(appName)
              if err != nil {
                     return err
              }
 
              finishCount := state.Result.Success + state.Result.Fail
              glog.Infof("DockerStatusRunning: %s, TaskSuccessCount: %d, TaskFinishCount: %d, TaskTotalCount: %d",
                     appName, state.Result.Success, finishCount, state.Count)
					 if state.Count == finishCount {
                     js, _ := json.Marshal(state)
                     glog.Infof("creating app state is: %s", string(js))
                     err = finishCall(state)
                     if err != nil {
                            glog.Error("CreatingState finish call fail: ", err)
                     }
                     err := c.CreatingState.DockerStatusDelete(appName)
                     if err != nil {
                            glog.Error("CreatingState.DockerStatusDelete fail: ", err)
                     }
              }
       case realstate.DockerStatusTimeOut:
              glog.Warning("DockerStatusTimeOut: ", appName)
              state, err := c.CreatingState.GetCreatingStatus(appName)
              if err != nil {
                     return err
              }
			  err = finishCall(state)
              if err != nil {
                     glog.Error("CreatingState finish call fail: ", err)
              }
              err = c.CreatingState.DockerStatusDelete(appName)
              if err != nil {
                     glog.Error("DockerStatusDelete error: ", err)
              }
       default:
              glog.Error("creating status fail")
       }
       return nil
}
 
func (c *SyncController) DropOneContainer(container *realstate.Container) {
       afterRun := func(err error) {
              result := realstate.Result{}
              state, erroccur := c.CreatingState.GetCreatingStatus(container.AppName)
              if erroccur != nil {
                     result.CreateUnixTime = time.Now().Unix()
              } else {
                     result.CreateUnixTime = state.CreateUnixTime
              }
			  if err == nil {
                     result.Success++
                     ME.NewEventReporter(ME.ExecResult, ME.ExecResultData{
                            Reason:   "Drop container success",
                            AppName:  container.AppName,
                            CtnrID:   container.ID,
                            Error:    "",
                            Birthday: time.Now(),
                            Message:  fmt.Sprintf("Drop Container %v Success --> Success Count: %v", container.ID, result.Success),
                     })
              } else {
                     result.Fail++
                     ME.NewEventReporter(ME.ExecResult, ME.ExecResultData{
                            Reason:   "Drop container failed",
                            AppName:  container.AppName,
                            CtnrID:   container.ID,
                            Error:    err.Error(),
							Birthday: time.Now(),
                            Message:  fmt.Sprintf("Drop Container %v Fail --> Fail Count: %v, %v", container.ID, result.Fail, err),
                     })
              }
              c.CreatingState.DockerStatusUpdateResult(container.AppName, result)
              ME.NewEventReporter(ME.DeleteOneContainerForAnyReason,
                     ME.DeleteOneContainerForAnyReasonData{
                            Container: *container,
                            Birthday:  time.Now(),
                            Message:   fmt.Sprintf("Update Job Success --> Success: %v, Fail: %v", result.Success, result.Fail),
                     })
       }
       executor.GetDockerManager().DockerExecAsync(executor.Drop, container, func() error { return nil }, afterRun)
       return
}

func (c *SyncController) RunOneContainer(app *app.App, deployInfo *node.DeployInfo, customAfterFunc func(err error)) {
       commonAfterRun := func(err error) {
              result := realstate.Result{}
              state, err := c.CreatingState.GetCreatingStatus(app.Name)
              if err != nil {
                     result.CreateUnixTime = time.Now().Unix()
              } else {
                     result.CreateUnixTime = state.CreateUnixTime
              }
 
              if err == nil {
                     result.Success  
              } else {
                     result.Fail  
              }
              err = c.CreatingState.DockerStatusUpdateResult(app.Name, result)
              if err != nil {
                     glog.Warning("CreatingState.DockerStatusUpdateResult fail: ", err)
              }
			}
       afterRun := func(err error) {
              customAfterFunc(err)
              commonAfterRun(err)
       }
       executor.GetDockerManager().DockerExecAsync(executor.Create, deployInfo, func() error { return nil }, afterRun)
}
 
func (c *SyncController) GetRealStateCTNRs(appName string) []*realstate.Container {
       containerlist := c.RealState.Containers(appName)
 
       //skip containers in special mode (maintenance ...)
       normalContainerList := []*realstate.Container{}
       fastsettingInstance, err := fastsetting.GetFastsetting()
       if err != nil {
              glog.Errorf("get fastsettingInstance err: %v", err)
              return normalContainerList
       }
	   for _, container := range containerlist {
              exist, err := fastsettingInstance.IsCTNRExecModeExist(container.ID)
              if nil != err {
                     glog.Errorf("IsCTNRExecModeExist: %s fail:%v", container.ID, err)
                     continue
              }
              if exist {
                     continue
              }
              normalContainerList = append(normalContainerList, container)
       }
       return normalContainerList
}
 
func (c *SyncController) GetRealStateCTNRsCount(appName string) int {
       containerlist := c.RealState.Containers(appName)
 
       //skip containers in special mode (maintenance ...)
       notUpgradedCtnrsCount := 0
       fastsettingInstance, err := fastsetting.GetFastsetting()
       if err != nil {
              glog.Errorf("get fastsettingInstance err: %v", err)
              return notUpgradedCtnrsCount
			  } 
       for _, container := range containerlist {
              exist, err := fastsettingInstance.IsCTNRExecModeExist(container.ID)
              if nil != err {
                     glog.Errorf("IsCTNRExecModeExist: %s fail:%v", container.ID, err)
                     continue
              }
              if exist {
                     continue
              }
              notUpgradedCtnrsCount++
       }
       return notUpgradedCtnrsCount
}
 
func (c *SyncController) UpdateAppStatus(app *app.App, status string, log string) error {
       if err := c.UpdateAppStatusByNameFunc(app.Name, status, log); err != nil {
              return err
       }
       app.Status = status
       return nil
}
