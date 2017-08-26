package factory
 
import (
       "errors"
       "fmt"
       "sync"
 
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm"
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm/filter"
       "github-beta.huawei.com/hipaas/scheduler/plugin/algorithm/strategy"
)
 
type ConfigFactory struct {
       getAllMinions        algorithm.GetAllMinionsFunc
       getAllVirtalMinions  algorithm.GetAllVirtalMinionsFunc
       getAppContainersFunc algorithm.GetAppAllContainersFunc
       dockerStatusUpdate   algorithm.DockerStatusUpdateFunc
}

func NewConfigFactory(getMinoins algorithm.GetAllMinionsFunc, getVirtalMinions algorithm.GetAllVirtalMinionsFunc, getContainers algorithm.GetAppAllContainersFunc, dockerStatusUpdate algorithm.DockerStatusUpdateFunc) *ConfigFactory {
       return &ConfigFactory{
              getAllMinions:        getMinoins,
              getAllVirtalMinions:  getVirtalMinions,
              getAppContainersFunc: getContainers,
              dockerStatusUpdate:   dockerStatusUpdate,
       }
}
 
// Create creates a scheduler with the default algorithm provider.
func (f *ConfigFactory) Create() (*algorithm.Config, error) {
       return f.CreateFromProvider(DefaultProvider)
}
// Creates a scheduler from the name of a registered algorithm provider.
func (f *ConfigFactory) CreateFromProvider(providerName string) (*algorithm.Config, error) {
       provider, err := GetAlgorithmProvider(providerName)
       if err != nil {
              return nil, err
       }
 
       return f.CreateFromKeys(provider.filterFunctionMap, provider.strategyFunctionMap)
}

// Creates a scheduler from a set of registered fit predicate keys and priority keys.
func (f *ConfigFactory) CreateFromKeys(filterFunctionMap map[string]filter.FilterFunc, strategyFunctionMap map[string]strategy.StrategyFunc) (*algorithm.Config, error) {
 
       algo := algorithm.NewGenericScheduler(filterFunctionMap, strategyFunctionMap, f.getAppContainersFunc, f.getAllMinions, f.getAllVirtalMinions, f.dockerStatusUpdate)
 
       return &algorithm.Config{
              Algorithm: algo,
       }, nil
}
 
type AlgorithmProviderMap struct {
       sync.RWMutex
       Provider map[string]AlgorithmProviderConfig
}
type AlgorithmProviderConfig struct {
       filterFunctionMap   map[string]filter.FilterFunc
       strategyFunctionMap map[string]strategy.StrategyFunc
}
 
var (
       algorithmProviderMap = AlgorithmProviderMap{
              Provider: make(map[string]AlgorithmProviderConfig),
       }
)
 
const (
       DefaultProvider = "DefaultProvider"
)
 
// Registers a new algorithm provider with the algorithm registry. This should
// be called from the init function in a provider plugin.
func RegisterAlgorithmProvider(name string, fitFilterMap map[string]filter.FilterFunc, fitStrategyMap map[string]strategy.StrategyFunc) error {
       algorithmProviderMap.Lock()
       defer algorithmProviderMap.Unlock()
 
       if "" == name {
              return errors.New("name can not be null")
       }
	   if nil == fitFilterMap || nil == fitStrategyMap {
              return errors.New("fitFilterMap or fitStrategyMap! name: "   name)
       }
 
       if _, ok := algorithmProviderMap.Provider[name]; ok {
              return errors.New("algorithm provider already exist! name: "   name)
       }
 
       algorithmProviderMap.Provider[name] = AlgorithmProviderConfig{
              filterFunctionMap:   fitFilterMap,
              strategyFunctionMap: fitStrategyMap,
       }
 
       return nil
}

func RegisterFitFilter(filters map[string]filter.FilterFunc, name string, filterFunc filter.FilterFunc) error {
       if "" == name {
              return fmt.Errorf("name can not be null")
       }
 
       if nil == filterFunc {
              return fmt.Errorf("filterFunc can not be null")
       }
 
       if nil == filters {
              return fmt.Errorf("filters can not be null")
       }
 
       if _, ok := filters[name]; ok {
              return fmt.Errorf("filter %q already been registered", name)
       }
 
       filters[name] = filterFunc
 
       return nil
 
}

func RegisterFitStrategy(strategy map[string]strategy.StrategyFunc, name string, strategyFunc strategy.StrategyFunc) error {
       if "" == name {
              return fmt.Errorf("name can not be null")
       }
 
       if nil == strategyFunc {
              return fmt.Errorf("strategyFunc can not be null")
       }
 
       if nil == strategy {
              return fmt.Errorf("filters can not be null")
       }
 
       if _, ok := strategy[name]; ok {
              return fmt.Errorf("filter %q already been registered", name)
       }
 
       strategy[name] = strategyFunc
       return nil
}

// This function should not be used to modify providers. It is publicly visible for testing.
func GetAlgorithmProvider(name string) (*AlgorithmProviderConfig, error) {
       algorithmProviderMap.RLock()
       defer algorithmProviderMap.RUnlock()
 
       var provider AlgorithmProviderConfig
       provider, ok := algorithmProviderMap.Provider[name]
       if !ok {
              return nil, fmt.Errorf("plugin %q has not been registered", name)
       }
 
       return &provider, nil
}
