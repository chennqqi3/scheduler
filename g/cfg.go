package g
 
import (
       "encoding/json"
       "errors"
       "flag"
       "fmt"
       "os"
       "strconv"
       "sync"
       "time"
 
       "github-beta.huawei.com/hipaas/common/redisoperate"
       "github-beta.huawei.com/hipaas/common/routes"
       "github-beta.huawei.com/hipaas/common/util/netutil"
       . "github-beta.huawei.com/hipaas/common/version"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/netservice/client"
       "github.com/toolkits/file"
)
 
const (
       //scheduler version.
       SCHEDULER_VERSION = "1.7.9"
       //scheduler compatible minimum minion version.
       MINION_MINIMUM_VERSION = "1.7.9"
)

var (
       config       *GlobalConfig
       configLock   = new(sync.RWMutex)
       netClient    *netservice.Client
       FlagInitOnce sync.Once
)
 
//add by huangjia 2015-07-13
//use to RPC Encrypt an dDecrypt Sign
type SignConfig struct {
       Switch   bool  `json:"switch"`
       Interval int64 `json:"interval"`
}
 
// HTTPConfig represent HTTP options.
type HTTPConfig struct {
       Addr string `json:"addr"`
       Port int    `json:"port"`
}
 
// RPCConfig represent RPC options.
type RPCConfig struct {
       Addr string `json:"addr"`
       Port int    `json:"port"`
}

// healthcheck options.
type HealthConfig struct {
       TimeOut  int `json:"timeout"`
       Interval int `json:"interval"`
       MaxTries int `josn:"maxtries"`
}
 
type MaintenanceMode struct {
       Switch bool `json:"switch"`
}
 
type ConsulConfig struct {
       ConsulAddress      string `json:"consuladdress"`
       ConsulScheme       string `json:"consulscheme"`
       ConsulDatacenter   string `json:"consuldatacenter"`
       CtnrToken          string `json:"ctnrtoken"`
       UpdateCtnrInterval int    `json:"updatectnrinterval"`
       ConsulRoutePath    string `json:"consulroutepath"`
       ConsulCtnrPath     string `json:"consulctnrpath"`
}
 
type ConsulSvcConfig struct {
       CtnrServicePrefix string   `json:"ctnrserviceprefix"`
       CtnrServiceTags   []string `json:"ctnrservicetags"`
}
 
type LogConfig struct {
       Verbose         int    `json:"verbose"`
       LogDir          string `json:"log_dir"`
       Alsologtostderr bool   `json:"alsologtostderr"`
}

type RecoveryReport struct {
       Switch      bool   `json:"switch"`
       RestAddress string `json:"restaddress"`
}
 
type Netservice struct {
       Switch  bool                     `josn:"switch"`
       Service *netservice.ClientConfig `json:"service"`
}
 
type DBConfig struct {
       Dsn         string `json:"dsn"`
       UserName    string `json:"userName"`
       Password    string `json:"password"`
       MaxIdle     int    `json:"maxIdle"`
       MaxOpen     int    `json:"maxOpen"`
       MaxLifetime int    `json:"maxlifetime"`
}
 
// GlobalConfig represent Global options.
// base on cfg.json
type GlobalConfig struct {
       Interval            int                       `json:"interval"`
       MaxDockerExecCount  int                       `json:"maxdockerexeccount"`
       LocalIP             string                    `json:"localIp"`
       BusinessNIC         string                    `json:"businessNIC"`
Redis               *redisoperate.RedisConfig `json:"redis"`
       DB                  *DBConfig                 `json:"db"`
       NetService          *Netservice               `json:"netservice"`
       HTTP                *HTTPConfig               `json:"http"`
       RPC                 *RPCConfig                `json:"rpc"`
       AgentRPCPort        int                       `json:"AgentRpcPort"`
       SignMsg             SignConfig                `json:"signMsg"`
       Health              *HealthConfig             `json:"health"`
       ConfigCheckDuration int                       `json:"configCheckDuration"`
       MAINT               MaintenanceMode           `json:"maintenancemode"`
       Route               *routes.Route             `json:"route"`
       ConsulServer        ConsulConfig              `json:"consulserver"`
       ConsulService       ConsulSvcConfig           `json:"consulservice"`
	   LogLevel            LogConfig                 `json:"logLevel"`
       RecoveryReport      *RecoveryReport           `json:"recoveryreport"`
       MinionMinVersion    *Versions                 `json:"minionminversion"`
}
 
func FlagInit() {
       FlagInitOnce.Do(func() {
              var c GlobalConfig
              flag.IntVar(&c.Interval, "interval", 5, "scheduler sync interval")
              flag.IntVar(&c.MaxDockerExecCount, "maxdockerexeccount", 2000, "maximum number of concurrent")
			  flag.StringVar(&c.BusinessNIC, "businessNIC", "eth0", "set the NIC to worked on")
              flag.IntVar(&c.ConfigCheckDuration, "configcheckduration", 5, "the interval to check the config file changes")
 
              //redis
              c.Redis = new(redisoperate.RedisConfig)
              flag.IntVar(&c.Redis.MaxIdle, "redis_maxidle", 700, "max idle connection to redis")
              flag.IntVar(&c.Redis.MaxActive, "redis_maxactive", 1000, "max active connection to redis")
              flag.BoolVar(&c.Redis.Wait, "redis_wait", true, "redis wait")
 
              //db
			  c.DB = new(DBConfig)
              flag.IntVar(&c.DB.MaxIdle, "mysql_maxidle", 100, "max idle connection to mysql")
              flag.IntVar(&c.DB.MaxOpen, "mysql_maxopen", 100, "max active connection to mysql")
              flag.IntVar(&c.DB.MaxLifetime, "mysql_maxlifetime", 300, "connection max lifetime to mysql")
 
              //http
              c.HTTP = new(HTTPConfig)
              flag.StringVar(&c.HTTP.Addr, "http_addr", "0.0.0.0", "the network interface address for scheduler http service")
              flag.IntVar(&c.HTTP.Port, "http_port", 1980, "the network interface port for scheduler http service")
 
              // rpc
			  c.RPC = new(RPCConfig)
              flag.StringVar(&c.RPC.Addr, "rpc_addr", "0.0.0.0", "the network interface address for scheduler rpc service")
              flag.IntVar(&c.RPC.Port, "rpc_port", 1970, "the network interface port for scheduler rpc service")
 
              // signMsg
              flag.BoolVar(&c.SignMsg.Switch, "signmsg_switch", true, "the signmsg switch")
              flag.Int64Var(&c.SignMsg.Interval, "signmsg_interval", 300, "the signmsg interval")
 
              // health
              c.Health = new(HealthConfig)
              flag.IntVar(&c.Health.TimeOut, "health_timeout", 10, "the app healthcheck timeout")
              flag.IntVar(&c.Health.Interval, "health_interval", 5, "the app healthcheck interval")
              flag.IntVar(&c.Health.MaxTries, "health_maxtries", 20, "the app healthcheck maxtries")
 
              // route
			  c.Route = new(routes.Route)
              flag.IntVar(&c.Route.PushInterval, "route_pushinterval", 60, "the route pushinterval")
              var routebackenddriver string
              flag.StringVar(&routebackenddriver, "route_backenddriver", "consul", "the route backendroutedrivers")
              c.Route.BackendRouteDrivers = append(c.Route.BackendRouteDrivers, routebackenddriver)
 
              //consulserver
              flag.StringVar(&c.ConsulServer.ConsulScheme, "consul_scheme", "http", "the consulserver scheme")
              flag.IntVar(&c.ConsulServer.UpdateCtnrInterval, "consul_updateinterval", 300, "the interval to update container info")
 
              //loglevel
              flag.BoolVar(&c.LogLevel.Alsologtostderr, "log_alsotostderr", false, "whether also print log to stderr")
			  
			  // version
              c.MinionMinVersion = new(Versions)
              c.NetService = new(Netservice)
 
              // recovery
              c.RecoveryReport = new(RecoveryReport)
              flag.BoolVar(&c.RecoveryReport.Switch, "recovery_report_switch", false, "whether report the failure app. " "if this is set to \"true\", then flag \"-recovery_report_restaddr\" must be configured with an available url.")
			  flag.StringVar(&c.RecoveryReport.RestAddress, "recovery_report_restaddr", "", "where to report the failure app, it's a url")
 
              configLock.Lock()
              defer configLock.Unlock()
              config = &c
       })
}

// add any necessary config check here
func (c *GlobalConfig) CheckDefault() {
       if c.Interval < 3 {
              c.Interval = 3
              glog.Warning("interval can not less than 3, set interval=3")
       }
 
       if c.MaxDockerExecCount < 400 {
              c.MaxDockerExecCount = 400
              glog.Warning("maxdockerexeccount can not less than 400, set maxdockerexeccount=400")
       }
 
       if c.ConfigCheckDuration < 5 {
              c.ConfigCheckDuration = 5
              glog.Warning("configcheckduration can not less than 5, set configcheckduration=5")
       }
 
       //redis
       if c.Redis.MaxIdle < 100 {
              c.Redis.MaxIdle = 100
              glog.Warning("redis_maxidle can not less than 100, set redis_maxidle=100")
       }
	   
	   if c.Redis.MaxActive < 600 {
              c.Redis.MaxActive = 600
              glog.Warning("redis_maxactive can not less than 600, set redis_maxactive=600")
       }
 
       //db
       if c.DB.MaxIdle < 20 {
              c.DB.MaxIdle = 20
              glog.Warning("mysql_maxidle can not less than 20, set mysql_maxidle=20")
       }
       if c.DB.MaxOpen < 20 {
              c.DB.MaxOpen = 20
              glog.Warning("mysql_maxopen can not less than 20, set mysql_maxopen=20")
       }
       if c.DB.MaxLifetime < 300 {
              c.DB.MaxLifetime = 300
              glog.Warning("mysql_maxlifetime can not less than 300, set mysql_maxlifetime=300")
       }
 
       //sign
       if c.SignMsg.Interval < 300 {
              c.SignMsg.Interval = 300
              glog.Warning("signmsg_interval can not less than 300, set signmsg_interval=300")
       }
	   
	   //health
       if c.Health.TimeOut < 10 {
              c.Health.TimeOut = 10
              glog.Warning("health_timeout can not less than 10, set health_timeout=10")
       }
       if c.Health.Interval < 5 {
              c.Health.Interval = 5
              glog.Warning("health_interval can not less than 5, set health_interval=5")
       }
       if c.Health.MaxTries < 10 {
              c.Health.MaxTries = 10
              glog.Warning("health_maxtries can not less than 10, set health_maxtries=10")
       }
 
       //route
       if c.Route.PushInterval < 30 {
              c.Route.PushInterval = 30
              glog.Warning("route_pushinterval can not less than 30, set route_pushinterval=30")
       }
	   
	   //consulServer
       if c.ConsulServer.UpdateCtnrInterval < 60 {
              c.ConsulServer.UpdateCtnrInterval = 60
              glog.Warning("consul_updateinterval can not less than 60, set consul_updateinterval=60")
       }
}
 
// Config returns global config.
func Config() GlobalConfig {
       configLock.RLock()
       defer configLock.RUnlock()
       return *config
}
 
// send maintence mode state to message engine
func updateMaintMode() {
       select {
       case MsgEngineStateChan <- Config().MAINT.Switch:
       default:
       }
}
 
func PrintlnAndReturnErr(errormessage string) error {
       glog.Error(errormessage)
       return errors.New(errormessage)
}

// ParseConfig parse config file.
func ParseConfig(cfg string) error {
       configLock.Lock()
       defer configLock.Unlock()
 
       err := config.MinionMinVersion.SetVersion(MINION_MINIMUM_VERSION)
       if nil != err {
              return PrintlnAndReturnErr(fmt.Sprintf("compatible minion minimum version is invalid: %v", err))
       }
 
       if cfg == "" {
              return PrintlnAndReturnErr("use -c to specify configuration file")
       }
       if !file.IsExist(cfg) {
              return PrintlnAndReturnErr(fmt.Sprintf("config file: %v is not existent", cfg))
       }
       configContent, err := file.ToTrimString(cfg)
       if err != nil {
              return PrintlnAndReturnErr(fmt.Sprintf("read config  file fail: %v ", err))
       }

// pick the fields we need from configuration file
       var c GlobalConfig
       err = json.Unmarshal([]byte(configContent), &c)
       if err != nil {
              return PrintlnAndReturnErr(fmt.Sprintf("parse config file: %v fail: %v ", cfg, err))
       }
 
       config.AgentRPCPort = c.AgentRPCPort
       config.LocalIP = c.LocalIP
 
       config.MAINT = c.MAINT
       config.ConsulService = c.ConsulService
       if c.NetService != nil {
              config.NetService = c.NetService
       }
 
       config.Redis.Dsn = c.Redis.Dsn
       config.Redis.PassWord = c.Redis.PassWord
       config.Redis.Sentinel = c.Redis.Sentinel
 
       config.DB.Dsn = c.DB.Dsn
       config.DB.UserName = c.DB.UserName
       config.DB.Password = c.DB.Password
	   
	   config.ConsulServer.ConsulAddress = c.ConsulServer.ConsulAddress
       config.ConsulServer.ConsulDatacenter = c.ConsulServer.ConsulDatacenter
       config.ConsulServer.CtnrToken = c.ConsulServer.CtnrToken
       config.ConsulServer.ConsulRoutePath = c.ConsulServer.ConsulRoutePath
       config.ConsulServer.ConsulCtnrPath = c.ConsulServer.ConsulCtnrPath
 
       config.LogLevel.LogDir = c.LogLevel.LogDir
       config.LogLevel.Verbose = c.LogLevel.Verbose
       // end of picking

flag.Lookup("Halsologtostderr").Value.Set(strconv.FormatBool(config.LogLevel.Alsologtostderr))
       flag.Lookup("Hv").Value.Set(strconv.Itoa(config.LogLevel.Verbose))
       if config.LogLevel.LogDir != "" {
              flag.Lookup("Hlog_dir").Value.Set(config.LogLevel.LogDir)
       }
       if config.LocalIP == "" && config.BusinessNIC == "" {
              return PrintlnAndReturnErr("localIp and businessNIC can not be empty at the same time")
       } else if config.LocalIP == "" {
              // detect local ip
              localIP, err := netutil.GetIPByEthPort(config.BusinessNIC)
              if err != nil {
                     return PrintlnAndReturnErr(fmt.Sprintf("get intranet ip fail: %v ", err))
              }
              config.LocalIP = localIP
       }
 
       if c.ConsulServer.ConsulRoutePath == "" {
              return PrintlnAndReturnErr("app consul route path can not be empty.")
       }
if c.ConsulServer.ConsulCtnrPath == "" {
              return PrintlnAndReturnErr("app consul container publish path can not be empty.")
       }
 
       glog.Infof("set app consul route path:%s", c.ConsulServer.ConsulRoutePath)
       glog.Infof("set app consul containerPublish path:%s", c.ConsulServer.ConsulCtnrPath)
 
       config.CheckDefault()
 
       js, _ := json.Marshal(*config)
       glog.Infof("Read config file: %v success. Config is: %v", cfg, string(js))
       return nil
}
 
func DyncCfg(cfg string) error {
       if cfg == "" {
              return errors.New("cfg file path is null.")
       }
       if !file.IsExist(cfg) {
              return PrintlnAndReturnErr(fmt.Sprintf("config file: %v is not existent", cfg))
       }
	   
	   configContent, err := file.ToTrimString(cfg)
       if err != nil {
              return PrintlnAndReturnErr(fmt.Sprintf("read config  file fail: %v ", err))
       }
 
       var c GlobalConfig
       err = json.Unmarshal([]byte(configContent), &c)
       if err != nil {
              return PrintlnAndReturnErr(fmt.Sprintf("parse config file: %v fail: %v ", cfg, err))
       }
 
       configLock.Lock()
       defer configLock.Unlock()
 
       if config.MAINT != c.MAINT {
              glog.Warningf("scheduler change run mode from %v to %v.", config.MAINT, c.MAINT)
              config.MAINT = c.MAINT
       }
 
       if config.LogLevel.Verbose != c.LogLevel.Verbose {
              glog.Warningf("log level verbose change from %d to %d. ", config.LogLevel.Verbose, c.LogLevel.Verbose)
              config.LogLevel.Verbose = c.LogLevel.Verbose
              flag.Set("Hv", strconv.Itoa(c.LogLevel.Verbose))
       }
       return nil
}

func WatchFile(file string) {
       preStat, err := os.Stat(file)
       if err != nil {
              glog.Error("get file state fail:", err)
       }
       updateMaintMode()
       for {
              nowStat, err := os.Stat(file)
              if err != nil {
                     glog.Error("get file state fail: ", err)
                     time.Sleep(time.Duration(Config().ConfigCheckDuration) * time.Second)
                     continue
              }
 
              if nowStat.ModTime() != preStat.ModTime() {
                     preStat = nowStat
                     if err := DyncCfg(file); nil != err {
                            glog.Error("DyncCfg failed. Error: ", err)
                     }
                     updateMaintMode()
              }
              time.Sleep(time.Duration(Config().ConfigCheckDuration) * time.Second)
       }
}

// if you use this function, please do assure that netClient is valid,
// please use this function along with Config().NetService.Switch switch.
func NetClient() *netservice.Client {
       return netClient
}
 
func NewNetClient() error {
       if !Config().NetService.Switch {
              return nil
       }
       var err error
       netClient, err = netservice.NewClient(Config().NetService.Service)
       if err != nil {
              return err
       }
       return nil
}

 
