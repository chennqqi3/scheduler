package g
 
import (
       "github-beta.huawei.com/hipaas/common/model"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/glog"
)
 
const (
       edcname string = "edc-1"
)
 
type Topology struct {
}
 
func (top *Topology) GetGlobalTopology() (*model.GlobalLevel, error) {
       // get all EDC
 
       // handle each edc
       edc := make(map[string]*model.EDCLevel)
       val, err := GetEdcLevel()
       if nil != err {
              return nil, err
       } else {
              edc[edcname] = val
       }
	   return &model.GlobalLevel{
                     EDC:            edc,
                     TopologyCommon: val.TopologyCommon,
              },
              nil
}
 
// each edc
func GetEdcLevel() (*model.EDCLevel, error) {
       // all the nodes
       nodes, err := RealNodeState.GetAllNode()
       if err != nil {
              glog.Error("get edc level fail: ", err)
              return nil, err
       }
       // there is no node
       if 0 >= len(nodes) {
              glog.Error("get edc level fail, there is no nodes.")
              return &model.EDCLevel{}, nil
       }
       // all the containers
       var conts []*realstate.Container
       for _, key := range RealState.Keys() {
              cs := RealState.Containers(key)
              for _, container := range cs {
                     conts = append(conts, container)
              }
       }
	   // handle all teh az
       azs := make(map[string][]*node.Node)
       for _, az := range nodes {
              azs[az.AvailableZone] = append(azs[az.AvailableZone], az)
       }
 
       // handle every az
       topocomm := model.TopologyCommon{
              Usage: &model.Resource{
                     Cpu:    0,
                     Memory: 0,
              },
              UnUsage: &model.Resource{
                     Cpu:    0,
                     Memory: 0,
              },
       }
 
       azlevel := make(map[string]*model.AZLevel)
       for aztype, nds := range azs {
              azlevel[aztype] = GetAZLevel(nds, conts)
              CalcTopoComm(&topocomm, &((*azlevel[aztype]).TopologyCommon))
       }
 
       return &model.EDCLevel{
                     AZ:             azlevel,
                     TopologyCommon: topocomm,
              },
              nil
}

func GetAZLevel(nodes []*node.Node, conts []*realstate.Container) *model.AZLevel {
       // all the vlan in a AZ
       topocomm, vlan := GetVlan(nodes, conts)
 
       // all the vmgroup in a AZ
       vmgrp := GetVmGrp(nodes, conts)
 
       return &model.AZLevel{
              Vlan:           vlan,
              VMGroup:        vmgrp,
              TopologyCommon: *topocomm,
       }
}
 
func GetVlan(nodes []*node.Node, conts []*realstate.Container) (*model.TopologyCommon, map[string]*model.VlanLevel) {
       vlans := make(map[string][]*node.Node)
 
       for _, nd := range nodes {
              vlans[nd.Region] = append(vlans[nd.Region], nd)
       }
	   topocomm := model.TopologyCommon{
              Usage: &model.Resource{
                     Cpu:    0,
                     Memory: 0,
              },
              UnUsage: &model.Resource{
                     Cpu:    0,
                     Memory: 0,
              },
       }
 
       vlan := make(map[string]*model.VlanLevel)
       for reg, vlNodes := range vlans {
              vlan[reg] = GetVlanLevel(vlNodes, conts)
              CalcTopoComm(&topocomm, &((*vlan[reg]).TopologyCommon))
       }
 
       return &topocomm, vlan
}
 
func GetVlanLevel(nodes []*node.Node, conts []*realstate.Container) *model.VlanLevel {
 
       vm := make(map[string][]*node.Node)
       for _, nd := range nodes {
              vm[nd.IP] = append(vm[nd.IP], nd)
       }
	   topocomm := model.TopologyCommon{
              Usage: &model.Resource{
                     Cpu:    0,
                     Memory: 0,
              },
              UnUsage: &model.Resource{
                     Cpu:    0,
                     Memory: 0,
              },
       }
       vms := make(map[string]*model.VMLevel)
       for ndip, vmNodes := range vm {
              vms[ndip] = GetVmLevel(vmNodes, conts)
              CalcTopoComm(&topocomm, &((*vms[ndip]).TopologyCommon))
       }
 
       return &model.VlanLevel{
              VM:             vms,
              TopologyCommon: topocomm,
       }
}
 
func GetVmLevel(nodes []*node.Node, conts []*realstate.Container) *model.VMLevel {
       containers := make(map[string]*realstate.Container)
       nodesmap := make(map[string]*node.Node)
       for _, nd := range nodes {
              nodesmap[nd.IP] = nd
       }
	   var cpu, mem, unCPU, unMem int
 
       for _, nd := range nodes {
              unCPU  = nd.CPU
              unMem  = nd.Memory
       }
 
       if len(conts) > 0 {
              for _, ct := range conts {
                     if _, ok := nodesmap[ct.IP]; ok {
                            containers[(*ct).ID] = ct
                            cpu  = ct.CPU
                            mem  = ct.Memory
                     }
              }
 
              unCPU -= cpu
              unMem -= mem
       }
 
       topocomm := model.TopologyCommon{
              Usage: &model.Resource{
                     Cpu:    cpu,
                     Memory: mem,
              },
              UnUsage: &model.Resource{
                     Cpu:    unCPU,
                     Memory: unMem,
              },
       }
	   return &model.VMLevel{
              Containers:     containers,
              TopologyCommon: topocomm,
       }
}
 
func GetVmGrp(nodes []*node.Node, conts []*realstate.Container) map[string]*model.VMGroupLevel {
       vms := make(map[string][]*node.Node)
       for _, nd := range nodes {
              vms[nd.VMType] = append(vms[nd.VMType], nd)
       }
 
       grps := make(map[string]*model.VMGroupLevel)
       for tp, nds := range vms {
              grps[tp] = GetVmGrpLevel(nds, conts)
       }
 
       return grps
}
 
func GetVmGrpLevel(nodes []*node.Node, conts []*realstate.Container) *model.VMGroupLevel {
       vm := make(map[string][]*node.Node)
       if 0 >= len(nodes) {
              return nil
       }
       for _, nd := range nodes {
              vm[nd.IP] = append(vm[nd.IP], nd)
       }
	   topocomm := model.TopologyCommon{
              Usage: &model.Resource{
                     Cpu:    0,
                     Memory: 0,
              },
              UnUsage: &model.Resource{
                     Cpu:    0,
                     Memory: 0,
              },
       }
       vms := make(map[string]*model.VMLevel)
       for ndip, vmNodes := range vm {
              vms[ndip] = GetVmLevel(vmNodes, conts)
              CalcTopoComm(&topocomm, &((*vms[ndip]).TopologyCommon))
       }
 
       // val := GetVmLevel(nodes, conts)
       // vm[nodes[0].IP] = val
       return &model.VMGroupLevel{
              VM:             vms,
              TopologyCommon: topocomm,
       }
}

func CalcTopoComm(tc1 *model.TopologyCommon, tc2 *model.TopologyCommon) {
       CalcResources(tc1.Usage, tc2.Usage)
       CalcResources(tc1.UnUsage, tc2.UnUsage)
}
 
func CalcResources(cr1 *model.Resource, cr2 *model.Resource) {
       (*cr1).Cpu  = (*cr2).Cpu
       (*cr1).Memory  = (*cr2).Memory
}
