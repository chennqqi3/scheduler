package strategy
 
import (
       "fmt"
       "strings"
 
       "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/glog"
)
 
// MinionPriority represents the priority of scheduling to a particular host, lower priority is better.
type MinionPriority struct {
       Node  *node.Node
       Count int
       Score int
}
 
type MinionPriorityList []*MinionPriority
type StrategyFunc func(app *app.App, existContainers *ContainerList, minions *MinionPriorityList) (*MinionPriorityList, error)
 
func (h MinionPriorityList) Len() int {
       return len(h)
}
 
func (h MinionPriorityList) Less(i, j int) bool {
       return h[i].Score < h[j].Score
}
 
func (h MinionPriorityList) Swap(i, j int) {
       h[i], h[j] = h[j], h[i]
}
 
type ContainerList []*realstate.Container
 
const (
       MaxScore         = 30
       ImageRepoScore   = MaxScore * 3  / 30
       ImageTagScore    = MaxScore * 2  / 30
       CpuUsageScore    = MaxScore * 5  / 30
       MemUsageScore    = MaxScore * 5  / 30
       MinionCountScore = MaxScore * 5  / 30
       MinionFailsScore = MaxScore * 10 / 30
)
 
func imageURLSplit(imageURL string) (repo string, tag string, err error) {
       srcImages := strings.Split(imageURL, ":")
 
       repoLen := len(srcImages)
       if repoLen < 2 {
              return "", "", fmt.Errorf("Error image url(%s), must have ':' split repo and tag ", imageURL)
       }
 
       for i := 0; i < repoLen-1; i++ {
              repo = repo + srcImages[i]
       }
       tag = srcImages[repoLen-1]
 
       return repo, tag, nil
}
 
func ImageStrategy(app *app.App, existContainers *ContainerList, minions *MinionPriorityList) (*MinionPriorityList, error) {
       if nil == minions {
              return nil, nil
       }
 
       srcRepo, srcTag, err := imageURLSplit(app.Image.DockerImageURL)
       if err != nil {
              return nil, err
       }
 
       for _, minionPriority := range *minions {
              for _, image := range minionPriority.Node.Images {
                     dstRepo, dstTag, err := imageURLSplit(image)
                     if err != nil {
                            glog.Errorf("minion: %s error image URL: %s", minionPriority.Node.IP, image)
                            continue
                     }
 
                     if srcRepo == dstRepo {
                            minionPriority.Score += ImageRepoScore
                            if srcTag == dstTag {
                                   minionPriority.Score += ImageTagScore
                            }
                     }
              }
       }
 
       return minions, nil
}
 
func AppAffinityStrategy(app *app.App, existContainers *ContainerList, minions *MinionPriorityList) (*MinionPriorityList, error) {
       if nil == app || existContainers == nil || minions == nil {
              return nil, fmt.Errorf("AppAffinityStrategy parameters error!")
       }
       maxCpu := 0
       maxMem := 0
       for _, minionPriority := range *minions {
              if minionPriority.Node.CPUVirtUsage > maxCpu {
                     maxCpu = minionPriority.Node.CPUVirtUsage
              }
              if minionPriority.Node.MemVirtUsage > maxMem {
                     maxMem = minionPriority.Node.MemVirtUsage
              }
       }
 
       if 0 == maxCpu || 0 == maxMem {
              return minions, nil
       }
       for _, minionPriority := range *minions {
              scoreCpu := CpuUsageScore * (1 - (minionPriority.Node.CPUVirtUsage / maxCpu))
              scoreMem := MemUsageScore * (1 - (minionPriority.Node.MemVirtUsage / maxMem))
              minionPriority.Score += (scoreCpu + scoreMem)
       }
 
       return minions, nil
}
 
func MinionAntiAffinityStrategy(app *app.App, existContainers *ContainerList, minions *MinionPriorityList) (*MinionPriorityList, error) {
       if nil == minions {
              return nil, fmt.Errorf("invalid MinionAntiAffinityStrategy parameter minions")
       }
       maxFails := 0
       for _, minionPriority := range *minions {
              if minionPriority.Node.Fails > maxFails {
                     maxFails = minionPriority.Node.Fails
              }
       }
 
       if 0 == maxFails {
              return minions, nil
       }
 
       for _, minionPriority := range *minions {
              minionPriority.Score += MinionFailsScore * (1 - minionPriority.Node.Fails / maxFails)
       }
 
       return minions, nil
}
 
func MinionAffinityStrategy(app *app.App, existContainers *ContainerList, minions *MinionPriorityList) (*MinionPriorityList, error) {
       if nil == app || existContainers == nil || minions == nil {
              return nil, fmt.Errorf("MinionAffinityStrategy parameters error!")
       }
 
       // ipCount := make(map[string]int)
 
       max := 0
 
       for _, minionPriority := range *minions {
              // mapAdd(ipCount, minionPriority.Node.IP, 1)
 
              if minionPriority.Count > max {
                     max = minionPriority.Count
              }
       }
 
       if 0 == max {
              return minions, nil
       }
 
       for _, minionPriority := range *minions {
              score := MinionCountScore * (minionPriority.Count) / max
              minionPriority.Score += score
       }
 
       return minions, nil
}
 
func mapAdd(selectCount map[string]int, key string, count int) {
       if _, ok := selectCount[key]; ok {
              selectCount[key]++
       } else {
              selectCount[key] = 1
       }
       return
}
