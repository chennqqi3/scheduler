package filter
 
import (
       "fmt"
       "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/scheduler/g"
       // "log"
)
 
const (
       UnDefineMax = 100
)
 
type FilterFunc func(app *app.App, node *node.Node) (bool, int, error)
 
func RegionFilter(app *app.App, node *node.Node) (bool, int, error) {
       if app.Region != "" && app.Region != node.Region {
              return false, UnDefineMax, nil
       }
 
       return true, UnDefineMax, nil
}
 
func VMTypeFilter(app *app.App, node *node.Node) (bool, int, error) {
       if app.VMType != "" && app.VMType != node.VMType {
              return false, UnDefineMax, nil
       }
 
       return true, UnDefineMax, nil
}
 
func CpuFilter(app *app.App, node *node.Node) (bool, int, error) {
       nodeFreeVirtCpu := node.CPU - node.CPUVirtUsage
       if app.CPU <= 0 {
              return false, UnDefineMax, fmt.Errorf("app.CPU can not less than 0")
       }
 
       if nodeFreeVirtCpu < app.CPU {
              return false, UnDefineMax, nil
       }
 
       count := nodeFreeVirtCpu / app.CPU
       return true, count, nil
}
 
func VirtMemoryFilter(app *app.App, node *node.Node) (bool, int, error) {
       if app.Memory <= 0 {
              return false, UnDefineMax, fmt.Errorf("app.Memory can not less than 0")
       }
 
       nodeFreeVirtMemory := node.Memory - node.MemVirtUsage
       if nodeFreeVirtMemory < app.Memory {
              return false, UnDefineMax, nil
       }
 
       count := nodeFreeVirtMemory / app.Memory
       return true, count, nil
}
 
func RealMemoryFileter(app *app.App, node *node.Node) (bool, int, error) {
       if app.Memory <= 0 {
              return false, UnDefineMax, fmt.Errorf("app.Memory can not less than 0")
       }
 
       if int(node.MemFree) < app.Memory {
              return false, UnDefineMax, nil
       }
 
       count := int(node.MemFree) / app.Memory
       return true, count, nil
}
 
func VersionFileter(app *app.App, node *node.Node) (bool, int, error) {
       if !g.Config().MinionMinVersion.IsVersionIncompatible(node.Version) {
              return false, UnDefineMax, nil
       }
       return true, UnDefineMax, nil
}
