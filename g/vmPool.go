package g
 
import (
       "github-beta.huawei.com/hipaas/common/storage/network"
       "time"
)
 
type VmPoolResource struct {
       Vlan           string
       VmTotal        float64
       ContainerTotal float64
       ContainerUsage float64
       DiskTotal      float64
       DiskUsed       float64
       DiskUsage      float64
       CpuTotal       float64
       CpuUsed        float64
       CpuUsage       float64
       MemoryTotal    float64
       MemoryUsed     float64
       MemoryUsage    float64
       IpTotal        float64
       IpUsed         float64
       IpUsage        float64
       TimeStamp      int64
       CreatedDate    string
}
 
func FindVmPoolResource() ([]VmPoolResource, error) {
       timestr := time.Now().Format("2006-01-02 15:04:05")
       if RealNodeState == nil {
              return nil, nil
       }
       nodes, err := RealNodeState.GetAllNode()
       if err != nil {
              return nil, err
       }
       if len(nodes) <= 0 {
              return nil, nil
       }
 
       netSources := make([]network.NetResource, 0)
       if Config().NetService.Switch {
              netSources, err = NetClient().GetNetWorksByRegion()
              if err != nil {
                     return nil, err
              }
       }
 
       nodeVlans := make(map[string]string)
       var vmPoolResources []VmPoolResource
 
       for _, node := range nodes {
              nodeVlans[node.Region] = node.Region
       }
 
       for _, nodeVlan := range nodeVlans {
              vmCount := 0
              var vmPoolResource VmPoolResource
              var containerUsed = 0
              var cpuTotal = 0
              var cpuUsed = 0
              var memoryTotal = 0
              var memoryUsed = 0
 
              for _, nodeValue := range nodes {
                     if nodeVlan == nodeValue.Region {
                            vmCount++
                            cpuTotal += nodeValue.CPU
                            cpuUsed += nodeValue.CPUVirtUsage
                            memoryTotal += nodeValue.Memory
                            memoryUsed += nodeValue.MemVirtUsage
 
                     }
              }
 
              for _, netSource := range netSources {
                     if netSource.Vlan == nodeVlan {
                            vmPoolResource.IpTotal = netSource.Total
                            vmPoolResource.IpUsed = netSource.Used
                            vmPoolResource.IpUsage = netSource.Usage
                     }
              }
 
              for _, key := range RealState.Keys() {
                     cs := RealState.Containers(key)
                     for _, container := range cs {
                            if _, ok := nodeVlans[container.Region]; ok {
                                   containerUsed++
                            }
                     }
              }
              vmPoolResource.Vlan = nodeVlan
              vmPoolResource.CreatedDate = timestr
              vmPoolResource.VmTotal = float64(vmCount)
              vmPoolResource.ContainerTotal = float64(cpuTotal)
              vmPoolResource.CpuTotal = float64(cpuTotal)
              vmPoolResource.CpuUsed = float64(cpuUsed)
              vmPoolResource.CpuUsage = float64(cpuUsed) / float64(cpuTotal)
              vmPoolResource.MemoryTotal = float64(memoryTotal)
              vmPoolResource.MemoryUsed = float64(memoryUsed)
              vmPoolResource.MemoryUsage = float64(memoryUsed) / float64(memoryTotal)
              vmPoolResource.ContainerUsage = float64(containerUsed) / float64(vmPoolResource.ContainerTotal)
              vmPoolResource.TimeStamp = time.Now().Unix()
              vmPoolResources = append(vmPoolResources, vmPoolResource)
       }
 
       return vmPoolResources, nil
}
