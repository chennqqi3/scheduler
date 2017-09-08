package g
 
import (
       "errors"
       "sort"
       "time"
 
       "github-beta.huawei.com/hipaas/common/storage/app"
       nodeStorage "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/glog"
)
 
func clone(regionName string) map[string]*nodeStorage.Node {
       ret := make(map[string]*nodeStorage.Node)
 
       if RealNodeState == nil {
              return ret
       }
 
       nodes, err := RealNodeState.GetAllNode()
       if err != nil {
              glog.Error("RealNodeState.GetAllNode fail: ", err)
              return ret
       }
 
       if regionName == "" {
              for _, n := range nodes {
                     ret[n.IP] = n
              }
       } else {
              for _, n := range nodes {
                     if n.Region == regionName {
                            ret[n.IP] = n
                     }
              }
       }
       return ret
}
 
// UpdateNode update node infomations.
func UpdateNode(node *nodeStorage.Node) {
       node.UpdateAt = time.Now().Unix()
 
       if RealNodeState == nil {
              return
       }
 
       err := RealNodeState.UpdateNode(node)
       if err != nil {
              glog.Error("RealNodeState.UpdateNode fail: ", err)
       }
 
       return
}
 
// DeleteStaleNode delete stale node.
func DeleteStaleNode(before int64) []string {
       if RealNodeState == nil {
              return nil
       }
 
       nodes, err := RealNodeState.GetAllNode()
       if err != nil {
              glog.Error("RealNodeState.GetAllNode fail: ", err)
              return nil
       }
 
       var deleteNodeIPs []string
       for _, node := range nodes {
              if node.UpdateAt < before {
                     glog.Warning("[HeartTimeout] delete node, ip: ", node.IP)
                     err := RealNodeState.DeleteNode(node.IP)
                     if err != nil {
                            glog.Error("RealNodeState.DeleteNode fail: ", err)
                     }
                     deleteNodeIPs = append(deleteNodeIPs, node.IP)
              }
       }
       return deleteNodeIPs
}
 
// DeleteNode delete Node.
func DeleteNode(ip string) error {
       if RealNodeState == nil {
              return errors.New("RealNodeState can't be null")
       }
 
       err := RealNodeState.DeleteNode(ip)
       if err != nil {
              glog.Error("RealNodeState.DeleteNode fail: ", err)
              return err
       }
 
       return nil
}
 
// TheOne select one node randomly.
func TheOne() *nodeStorage.Node {
       if RealNodeState == nil {
              return nil
       }
 
       nodes, err := RealNodeState.GetAllNode()
       if err != nil {
              glog.Error("RealNodeState.GetAllNode fail: ", err)
              return nil
       }
 
       for _, n := range nodes {
              return n
       }
 
       return nil
}
 
// GetNode return the node info.
func GetNode(ip string) *nodeStorage.Node {
 
       if RealNodeState == nil {
              return nil
       }
 
       node, exist := RealNodeState.GetNode(ip)
       if !exist {
              return nil
       }
 
       return node
}
 
var ErrNodeNotExist = errors.New("node not exist!")
var ErrRealNodeStateNotExist = errors.New("RealNodeState not exist!")
 
// GetRegionByNode select available nodes to deploy.
func GetRegionByNode(ip string) (string, error) {
 
       if RealNodeState == nil {
              return "", ErrRealNodeStateNotExist
       }
 
       node, exist := RealNodeState.GetNode(ip)
       if !exist {
              return "", ErrNodeNotExist
       }
 
       return node.Region, nil
}
 
// ChooseNode select available nodes to deploy.
// node election algorithm
func ChooseNode(app *app.App, region string, deployCnt int) map[string]int {
       // key:IP, value:instance numbers
       ret := make(map[string]int)
 
       copyNodes := clone(region)
       size := len(copyNodes)
       if size == 0 {
              return ret
       }
 
       if size == 1 {
              n := TheOne()
              if n != nil && n.MemFree > uint64(deployCnt*app.Memory) {
                     ret[n.IP] = deployCnt
                     return ret
              }
              glog.Warning("[WARNING] memory not enough for %d instance of %s", app.Instance, app.Name)
              return ret
       }
 
       // order by MemFree desc
       ns := make(nodeStorage.NodeSlice, 0, size)
       for _, n := range copyNodes {
              ns = append(ns, n)
       }
 
       sort.Sort(ns)
 
       // delete node which MemFree < app.Memory
       memFreeIsOK := make([]*nodeStorage.Node, 0, size)
       for _, n := range ns {
              if n.MemFree > uint64(app.Memory) {
                     memFreeIsOK = append(memFreeIsOK, n)
              }
       }
 
       size = len(memFreeIsOK)
       if size == 0 {
              return ret
       }
 
       // node not enough
       if size < deployCnt {
 
              // every node at least create one container
              for _, n := range memFreeIsOK {
                     ret[n.IP] = 1
              }
 
              done := len(memFreeIsOK)
              for {
                     for _, n := range memFreeIsOK {
                            ret[n.IP]++
                            done++
                            if done == deployCnt {
                                   goto CHK_MEM
                            }
                     }
              }
 
       CHK_MEM:
 
              for _, n := range memFreeIsOK {
                     if n.MemFree < uint64(app.Memory*ret[n.IP]) {
                            glog.Warning("[WARNING] memory not enough for %d instance of %s", app.Instance, app.Name)
                            return make(map[string]int)
                     }
              }
 
              return ret
       }
 
       if size == deployCnt {
              for _, n := range memFreeIsOK {
                     ret[n.IP] = 1
              }
              return ret
       }
 
       // node enough
       hasDeployedCount := app.Instance - deployCnt
       if hasDeployedCount == 0 {
              // first deploy
              done := 0
              for _, n := range memFreeIsOK {
                     ret[n.IP] = 1
                     done++
                     if done == deployCnt {
                            return ret
                     }
              }
       }
 
       // we have enough nodes. delete the node which has deployed this app.
       // we can delete a maximum of size - deployCnt
       canDeleteNodeCount := size - deployCnt
       // the nodes not deploy this app, order by MemFree asc
       var notDeploythisApp []*nodeStorage.Node
 
       allOk := false
       for i := size - 1; i >= 0; i-- {
              if allOk {
                     notDeploythisApp = append(notDeploythisApp, memFreeIsOK[i])
                     continue
              }
 
              if !RealState.HasRelation(app.Name, memFreeIsOK[i].IP) {
                     notDeploythisApp = append(notDeploythisApp, memFreeIsOK[i])
                     continue
              }
 
              if canDeleteNodeCount > 0 {
                     hasDeployedCount--
                     if hasDeployedCount == 0 {
                            // the rest nodes are all not deploy this app
                            allOk = true
                            continue
                     }
                     canDeleteNodeCount--
              } else {
                     notDeploythisApp = append(notDeploythisApp, memFreeIsOK[i])
                     allOk = true
              }
       }
 
       // order by MemFree desc
       cnt := 0
       for i := len(notDeploythisApp) - 1; i >= 0; i-- {
              ret[notDeploythisApp[i].IP] = 1
              cnt++
              if cnt == deployCnt {
                     return ret
              }
       }
 
       return make(map[string]int)
}
 
func saveNodetoVlan(nodeSlice *nodeStorage.NodeSelectSlice, node *nodeStorage.Node, count int) {
       *nodeSlice = append(*nodeSlice, &nodeStorage.NodeSelect{
              Node:  node,
              Count: count,
       })
 
       return
}
 
func mapAdd(selectCount map[string]int, key string, count int) {
       if _, ok := selectCount[key]; ok {
              selectCount[key]++
       } else {
              selectCount[key] = 1
       }
       return
}
 
const (
       cRealNodeState      = "get_real_state_error"
       cParameter          = "parameter_error"
       cGetAllNode         = "get_node_error"
       cRegion             = "region_notmatch"
       cVmType             = "VmType_notmatch"
       cNodeFreeVirtCpu    = "nodeFreeVirtCpu_notmatch"
       cNodeFreeVirtMemory = "nodeFreeVirtMemory_notmatch"
       cMemFree            = "MemFree_notmatch"
       cSuccess            = "success"
)
 
// SelectNode select available nodes to deploy.
func NewSelectNode(app *app.App, region string) (nodeSlice *nodeStorage.NodeSelectSlice, selectCount map[string]int, err error) {
 
       nodeSlice = &nodeStorage.NodeSelectSlice{}
       selectCount = make(map[string]int)
 
       if app.CPU <= 0 || app.Memory <= 0 {
              mapAdd(selectCount, cParameter, 1)
              return nil, selectCount, errors.New("CPU and Memory must be positive!")
       }
 
       if RealNodeState == nil {
              mapAdd(selectCount, cRealNodeState, 1)
              return nil, selectCount, ErrRealNodeStateNotExist
       }
 
       allNodes, err := RealNodeState.GetAllNode()
       if err != nil {
              mapAdd(selectCount, cGetAllNode, 1)
              return nodeSlice, selectCount, err
       }
 
       nodeCount := len(allNodes)
       if 0 == nodeCount {
              mapAdd(selectCount, cGetAllNode, 1)
              return nodeSlice, selectCount, nil
       }
 
       for _, node := range allNodes {
 
              // Region要求
              if region != "" && region != node.Region {
                     mapAdd(selectCount, cRegion, 1)
                     continue
              }
 
              // VM类型要求
              if app.VMType != "" && app.VMType != node.VMType {
                     mapAdd(selectCount, cVmType, 1)
                     continue
              }
 
              // CPU槽位要求
              nodeFreeVirtCpu := node.CPU - node.CPUVirtUsage
              if nodeFreeVirtCpu < app.CPU {
                     mapAdd(selectCount, cNodeFreeVirtCpu, 1)
                     continue
              }
              cpuCount: = nodeFreeVirtCpu / app.CPU
              count: = cpuCount
 
              // memory slot requirements
              nodeFreeVirtMemory: = node.Memory - node.MemVirtUsage
              if nodeFreeVirtMemory <app.Memory {
                     mapAdd (selectCount, cNodeFreeVirtMemory, 1)
                     continue
              }
              memVirtCount: = nodeFreeVirtMemory / app.Memory
              if memVirtCount <count {
                     count = memVirtCount
              }
 
              // actual memory requirements
              if int (node.MemFree) <app.Memory {
                     mapAdd (selectCount, cMemFree, 1)
                     continue
              }
              memCount: = int (node.MemFree) / app.Memory
              if memCount < count {
                     count = memCount
              }
 
              if count > 0 {
                     mapAdd(selectCount, cSuccess, 1)
                     saveNodetoVlan(nodeSlice, node, count)
              }
       }
 
       return nodeSlice, selectCount, nil
}
 
// SelectNode select available nodes to deploy.
func SelectNode(app *app.App, allNodes []*nodeStorage.Node, region string) (nodeSlice *nodeStorage.NodeSelectSlice, selectCount map[string]int, err error) {
 
       nodeSlice = &nodeStorage.NodeSelectSlice{}
       selectCount = make(map[string]int)
 
       if app.CPU <= 0 || app.Memory <= 0 {
              mapAdd(selectCount, cParameter, 1)
              return nil, selectCount, errors.New("CPU and Memory must be positive!")
       }
 
       nodeCount := len(allNodes)
       if 0 == nodeCount {
              mapAdd(selectCount, cGetAllNode, 1)
              return nodeSlice, selectCount, nil
       }
 
       for _, node := range allNodes {
 
              // Region要求
              if region != "" && region != node.Region {
                     mapAdd(selectCount, cRegion, 1)
                     continue
              }
 
              // VM类型要求
              if app.VMType != "" && app.VMType != node.VMType {
                     mapAdd(selectCount, cVmType, 1)
                     continue
              }
 
              // CPU槽位要求
              nodeFreeVirtCpu := node.CPU - node.CPUVirtUsage
              if nodeFreeVirtCpu < app.CPU {
                     mapAdd(selectCount, cNodeFreeVirtCpu, 1)
                     continue
              }
              cpuCount: = nodeFreeVirtCpu / app.CPU
              count: = cpuCount
 
              // memory slot requirements
              nodeFreeVirtMemory: = node.Memory - node.MemVirtUsage
              if nodeFreeVirtMemory <app.Memory {
                     mapAdd (selectCount, cNodeFreeVirtMemory, 1)
                     continue
              }
              memVirtCount: = nodeFreeVirtMemory / app.Memory
              if memVirtCount <count {
                     count = memVirtCount
              }
 
              // actual memory requirements
              if int (node.MemFree) <app.Memory {
                     mapAdd (selectCount, cMemFree, 1)
                     continue
              }
              memCount: = int (node.MemFree) / app.Memory
              if memCount <count {
                     count = memCount
              }
 
              if count> 0 {
                     mapAdd (selectCount, cSuccess, 1)
                     saveNodetoVlan(nodeSlice, node, count)
              }
       }
 
       return nodeSlice, selectCount, nil
}
 
// ChooseNode select available nodes to deploy.
// node election algorithm
func NewChooseNode(nodeSlice *nodeStorage.NodeSelectSlice, app *app.App, deployCnt int) map[string]int {
       // key:IP, value:instance numbers
       ret := make(map[string]int)
       allNodeSlice := *nodeSlice
       size := len(allNodeSlice)
       if size == 0 {
              return ret
       }
 
       sort.Sort(allNodeSlice)
 
       for i := 0; i < deployCnt; {
 
              isSelect := false
              for _, nodeSelect := range allNodeSlice {
                     if nodeSelect.Count <= 0 {
                            break
                     }
 
                     _, ok := ret[nodeSelect.Node.IP]
                     if !ok {
                            ret[nodeSelect.Node.IP] = 1
                     } else {
                            ret[nodeSelect.Node.IP] += 1
                     }
 
                     isSelect = true
                     nodeSelect.Count -= 1
 
                     i++
                     if i == deployCnt {
                            return ret
                     }
              }
 
              if isSelect == false {
                     glog.Warning("[WARNING] source not enough for %d instance of %s", deployCnt, app.Name)
                     return ret
              }
       }
 
       return ret
}
 
type NodeSelect struct {
       Node  *nodeStorage.Node
       Count int
}
 
type RegionSelect struct {
       Nodes []*NodeSelect
       Count int
}
 
type DataCenterSelect map[string]*RegionSelect
 
func saveNode(dcSelect DataCenterSelect, node *nodeStorage.Node, count int) {
       if node == nil {
              return
       }
 
       region, ok := dcSelect[node.Region]
       if !ok {
              nodeSelect := &NodeSelect{
                     Node:  node,
                     Count: count,
              }
 
              regionSelect := &RegionSelect{
                     Nodes: []*NodeSelect{
                            nodeSelect,
                     },
                     Count: count,
              }
 
              dcSelect[node.Region] = regionSelect
 
              return
       }
 
       region.Count = region.Count + count
       region.Nodes = append(region.Nodes, &NodeSelect{
              Node:  node,
              Count: count,
       })
 
       return
}
