package defaults
 
import (
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm/factory"
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm/filter"
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm/strategy"
)
 
func init() {
       factory.RegisterAlgorithmProvider(factory.DefaultProvider, defaultFilters(), defaultStrategies())
}
 
func defaultFilters() map[string]filter.FilterFunc {
       filters := make(map[string]filter.FilterFunc)
       factory.RegisterFitFilter(filters, "RegionFilter", filter.RegionFilter)
       factory.RegisterFitFilter(filters, "VMTypeFilter", filter.VMTypeFilter)
       factory.RegisterFitFilter(filters, "CpuFilter", filter.CpuFilter)
       factory.RegisterFitFilter(filters, "VirtMemoryFilter", filter.VirtMemoryFilter)
	   factory.RegisterFitFilter(filters, "VersionFilter", filter.VersionFileter)
       // factory.RegisterFitFilter(filters, "RealMemoryFileter", filter.RealMemoryFileter)
 
       return filters
}
 
func defaultStrategies() map[string]strategy.StrategyFunc {
       strategies := make(map[string]strategy.StrategyFunc)
       factory.RegisterFitStrategy(strategies, "ImageStrategy", strategy.ImageStrategy)
       factory.RegisterFitStrategy(strategies, "MinionAffinityStrategy", strategy.MinionAffinityStrategy)
       factory.RegisterFitStrategy(strategies, "MinionAntiAffinityStrategy", strategy.MinionAntiAffinityStrategy)
       factory.RegisterFitStrategy(strategies, "AppAffinityStrategy", strategy.AppAffinityStrategy)
       return strategies
}
