package algorithm
 
import (
       // "encoding/json"
       "fmt"
       "sort"
       "sync"
 
       "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       // "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm/filter"
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm/strategy"
)
 
const (
       StatusDeleteTimeOut    = 600
       StatusDeleteTimeAgeing = StatusDeleteTimeOut * 2
       StatusCreateTimeOut    = 600
       StatusCreateTimeAgeing = StatusCreateTimeOut * 2
)

type MachineCount map[string]int
type FailedFilterMap map[string]string
type GetAppAllContainersFunc func(appName string) (cs []*realstate.Container)
type GetAllMinionsFunc func() ([]*node.Node, error)
type GetAllVirtalMinionsFunc func() (nodes map[string]*realstate.VirtualNode, err error)
type DockerStatusUpdateFunc func(appname string, count int, nodes []*realstate.VirtualNode, timeOut int64, timeAgeing int64) error
 
type Config struct {
       Algorithm ScheduleAlgorithm
}
 
type ScheduleAlgorithm interface {
       Schedule(app *app.App, deployCnt int) (count MachineCount, err error)
}
type genericScheduler struct {
       filterFunctionMap    map[string]filter.FilterFunc
       strategyFunctionMap  map[string]strategy.StrategyFunc
       getAppContainersFunc GetAppAllContainersFunc
       getAllMinions        GetAllMinionsFunc
       getAllVirtalMinions  GetAllVirtalMinionsFunc
       Mutex                sync.RWMutex
       dockerStatusUpdate   DockerStatusUpdateFunc
}

func NewGenericScheduler(filters map[string]filter.FilterFunc, strategies map[string]strategy.StrategyFunc, getContainers GetAppAllContainersFunc, getMinions GetAllMinionsFunc, getVirtalMinions GetAllVirtalMinionsFunc, dockerStatusUpdate DockerStatusUpdateFunc) ScheduleAlgorithm {
       return &genericScheduler{
              filterFunctionMap:    filters,
              strategyFunctionMap:  strategies,
              getAppContainersFunc: getContainers,
              getAllMinions:        getMinions,
              getAllVirtalMinions:  getVirtalMinions,
              dockerStatusUpdate:   dockerStatusUpdate,
       }
}
 
var ErrDeployCnt = fmt.Errorf("deployCnt is less than 0")

func (scheduler *genericScheduler) Schedule(app *app.App, deployCnt int) (count MachineCount, err error) {
       if deployCnt <= 0 {
              return MachineCount{}, ErrDeployCnt
       }
       scheduler.Mutex.Lock()
       defer scheduler.Mutex.Unlock()
       minions, err := scheduler.ListMinions()
       if err != nil {
              return MachineCount{}, err
       }
 
       filteredNodes, failedPredicateMap, filterCount, err := findNodesThatFit(app, scheduler.filterFunctionMap, minions)
       if err != nil {
              return MachineCount{}, err
       }
	   
	   if filterCount < deployCnt {
              errString := "fail: "
              for k, _ := range failedPredicateMap {
                     errString = fmt.Sprintf("%s ", k)
              }
              return MachineCount{}, fmt.Errorf("can not deploy, expect: %d, resouce: %d, fail: %s", deployCnt, filterCount, errString)
       }
 
       // glog.Debugf("expect: %d, resouce: %d", deployCnt, filterCount)
       existContainers := scheduler.getAppContainersFunc(app.Name)
       priorityList, err := strategyNodes(app, scheduler.strategyFunctionMap, filteredNodes, existContainers)
       if err != nil {
              return MachineCount{}, err
       }
 
       // priority, _ := json.Marshal(priorityList)
       // glog.Debugf("priorityList: %s ", string(priority))
       ipCount, err := GetBestHosts(priorityList, deployCnt)
       if err != nil {
              return MachineCount{}, err
       }
	   virtualNodes := []*realstate.VirtualNode{}
       for ip, count := range ipCount {
              virtualNodes = append(virtualNodes, &realstate.VirtualNode{
                     IP:     ip,
                     Cpu:    app.CPU * count,
                     Memory: app.Memory * count,
              })
       }
       err = scheduler.dockerStatusUpdate(app.Name, deployCnt,
              virtualNodes, StatusCreateTimeOut, StatusCreateTimeAgeing)
       if err != nil {
              return MachineCount{}, err
       }
 
       return ipCount, nil
}
 
func (scheduler *genericScheduler) ListMinions() (nodes []*node.Node, err error) {
 
       // []*node.Node
       allNodes, err := scheduler.getAllMinions()
       if err != nil {
              return nodes, err
       }
	   // map[string]*executor.VirtualNode
       allVirtalNodes, err := scheduler.getAllVirtalMinions()
       if err != nil {
              return nodes, err
       }
 
       for _, node := range allNodes {
              if val, ok := allVirtalNodes[node.IP]; ok {
                     node.CPUVirtUsage = node.CPUVirtUsage   val.Cpu
                     node.MemVirtUsage = node.MemVirtUsage   val.Memory
              }
       }
 
       return allNodes, nil
}
 
func GetBestHosts(priorityLists *strategy.MinionPriorityList, deployCnt int) (MachineCount, error) {
       if deployCnt <= 0 {
              return MachineCount{}, ErrDeployCnt
       }
	   machineCount := MachineCount{}
       sort.Sort(sort.Reverse(*priorityLists))
       /*****************此处需要优化，az均匀部署算法********************/
       count := 0
       azCount := map[string]int{}
       azDeploy := map[string]int{}
       azNum := 0
       total := 0
       for _, minionPriority := range *priorityLists {
              total = total   minionPriority.Count
              if _, exist := azCount[minionPriority.Node.AvailableZone]; !exist {
                     azCount[minionPriority.Node.AvailableZone] = minionPriority.Count
                     azNum  
              } else {
                     azCount[minionPriority.Node.AvailableZone] = azCount[minionPriority.Node.AvailableZone]   minionPriority.Count
              }
       }
	   if total < deployCnt {
              return MachineCount{}, fmt.Errorf("total count is %d but you want to deploy %d, not enough", total, deployCnt)
       }
       maxcount := 0
       exit := 0
       for {
              for key, val := range azCount {
                     if val <= 0 {
                            continue
                     }
                     if _, exist := azDeploy[key]; !exist {
                            azDeploy[key] = 1
                     } else {
                            azDeploy[key] = azDeploy[key] + 1
                     }
                     azCount[key] = val - 1
                     maxcount++
                     if maxcount >= deployCnt {
                            exit = 1
                            break
                     }
              }
 
              if exit == 1 {
                     break
              }
       }
       /*****************此处需要优化，az均匀部署算法********************/
	   for {
              isSelect := false
 
              for _, minionPriority := range *priorityLists {
 
                     if minionPriority.Count <= 0 {
                            continue
                     }
                     if azDeploy[minionPriority.Node.AvailableZone] <= 0 {
                            continue
                     }
                     machineCount[minionPriority.Node.IP] += 1
                     azDeploy[minionPriority.Node.AvailableZone] -= 1
                     minionPriority.Count--
                     isSelect = true
 
                     count++
                     if count == deployCnt {
                            return machineCount, nil
                     }
 
              }
 
              if false == isSelect {
                     return MachineCount{}, fmt.Errorf("must error, node: %d is not enough!", deployCnt)
              }
       }
 
       // return machineCount, nil
 
}

func strategyNodes(app *app.App, strategyMap map[string]strategy.StrategyFunc, priorityLists *strategy.MinionPriorityList, existContainers strategy.ContainerList) (*strategy.MinionPriorityList, error) {
 
       for _, strategy := range strategyMap {
              priorityLists, err := strategy(app, &existContainers, priorityLists)
              if err != nil {
                     return priorityLists, err
              }
       }
 
       return priorityLists, nil
}
 
func findNodesThatFit(app *app.App, filterMap map[string]filter.FilterFunc, minions []*node.Node) (*strategy.MinionPriorityList, FailedFilterMap, int, error) {
       successFiltered := strategy.MinionPriorityList{}
       failedPredicateMap := FailedFilterMap{}
       allCount := 0
	   for _, minion := range minions {
              minionCount := filter.UnDefineMax
 
              tmpPriority := strategy.MinionPriority{
                     Node:  minion,
                     Count: minionCount,
                     Score: 0,
              }
 
              isSuccess := true
              for name, filter := range filterMap {
                     fit, count, err := filter(app, minion)
                     if err != nil {
                            return &strategy.MinionPriorityList{}, nil, 0, err
                     }
 
                     if !fit {
                            mapStringAdd(failedPredicateMap, name "_failed", minion.IP)
                            isSuccess = false
                            break
                     }
 
                     if minionCount > count {
                            minionCount = count
                     }
              }
			  if isSuccess {
                     tmpPriority.Count = minionCount
 
                     allCount  = minionCount
                     successFiltered = append(successFiltered, &tmpPriority)
              }
       }
 
       return &successFiltered, failedPredicateMap, allCount, nil
}
 
func mapStringAdd(selectCount map[string]string, key string, src string) {
       if _, ok := selectCount[key]; ok {
              selectCount[key] = selectCount[key]   ","   src
       } else {
              selectCount[key] = src
       }
       return
}
