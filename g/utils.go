package g
 
import (
       "fmt"
       "time"
 
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/glog"
)
 
func ProcessStaleContainers(mapAppnameCtnrs map[string][]*realstate.Container) {
       for name, containers := range mapAppnameCtnrs {
              if len(containers) < 1 {
                     continue
              }
              brokenContainers := []string{}
              if containers[0].AppType != app.AppTypeLRC {
                     for _, container := range containers {
                            glog.Warningf(`app: %s, id: %s, container down, hostname: %s`, name, container.ID, container.Hostname)
                            RealState.DeleteContainer(container.AppName, container)
                     }
                     continue
              }

for _, container := range containers {
                     glog.Warningf(`app: %s, id: %s, container down, need recovery from failure!`, name, container.ID)
                     brokenContainers = append(brokenContainers, container.ID)
                     RealState.DeleteContainer(container.AppName, container)
              }
 
              status, err := GetStatusByAppName(name)
              if status != app.AppStatusStartSuccess {
                     if err != nil {
                            glog.Warningf("g.GetStatusByAppName fail, name: %s, err: %v", name, err)
                     }
                     continue
              }
			  
			  // in recovery
              UpdateRecoveryByName(name, true)
              UpdateAppStatusByName(name, app.AppStatusPending, "container need recovery from failure")
              for _, container := range containers {
                     ME.NewEventReporter(ME.Failover, ME.FailoverData{
                            Container: *container,
                            Birthday:  time.Now(),
                            Message:   fmt.Sprintf("App %v Recovery from Failure  --> broken containers: %v", name, brokenContainers),
                     })
              }
       }
}
 
func DeleteContainerByIP(ip string) {
       mapAppNameCtnrs := RealState.GetContainersMapByNodeIP(ip)
       ProcessStaleContainers(mapAppNameCtnrs)
}

