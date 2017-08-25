package g_test
 
import (
       "encoding/json"
       "os"
       // "fmt"
       "time"
       "flag"
 
       "github-beta.huawei.com/hipaas/common/redisoperate"
       // "github-beta.huawei.com/hipaas/common/mysqloper"
       "github-beta.huawei.com/hipaas/netservice/client"
       // "github-beta.huawei.com/hipaas/common/routes"
 
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
       . "github-beta.huawei.com/hipaas/scheduler/g"
)

type testConfigST struct {
       LocalIP             string                    `json:"localIp"`
       Redis               *redisoperate.RedisConfig `json:"redis"`
       DB                  *DBConfig                 `json:"db"`
       NetService          *Netservice               `json:"netservice"`
       AgentRPCPort        int                       `json:"AgentRpcPort"`
       MAINT               MaintenanceMode           `json:"maintenancemode"`
       ConsulServer        ConsulConfig              `json:"consulserver"`
       ConsulService       ConsulSvcConfig           `json:"consulservice"`
       LogLevel            LogConfig                 `json:"logLevel"`
}
 
const default_cfgfile = "testcfg.json"
 
var _ = Describe("cfg test",func(){
       var (
              g_testcfg testConfigST
       )
	   
	   // create cfg file
       create_configfile := func(cfg interface{},file string) {
              str,err := json.Marshal(cfg)
              Expect(err).NotTo(HaveOccurred())
 
              f,_ := os.Create(file)
              defer f.Close()
              f.Write(str)
       }
 
       BeforeEach(func(){
              g_testcfg = testConfigST {
                     LocalIP: "127.0.0.1",
                     AgentRPCPort: 1990,
                     Redis: &redisoperate.RedisConfig {
                            Dsn: "127.0.0.1:6379",
                            PassWord: "",
                            Sentinel: redisoperate.SntlCfg {
                                   Use: false,
                                   MasterName: "mymaster",
                                   SntlAddrs: []string{"127.0.0.1:26379"},
								   },
                     },
                     DB: &DBConfig {
                            Dsn: "@tcp(127.0.0.1:3306)/hipaas?loc=Local&parseTime=true",
                            UserName: "root",
                            Password: "MBTWaNOTW35lhjYGOtJy4A==",               
                     },
                     NetService:     &Netservice {
                            Switch: true,
                            Service: &netservice.ClientConfig {
                                   Host: "localhost",
                                   Port: 8889,
                                   CaCrtFile: "ca.crt",
                                   Auth: false,
                                   CrtFile: "netservice_Client.crt",
                                   KeyFile: "netservice_Client.key",
                            },
                     },
					 
					 MAINT: MaintenanceMode {
                            Switch: false,
                     },
                     ConsulServer: ConsulConfig {
                            ConsulAddress: "127.0.0.1:8500",
                            ConsulDatacenter: "testDC",
                            CtnrToken: "",
                     },
                     ConsulService: ConsulSvcConfig {
                            CtnrServicePrefix: "unitTest_container_",
                            CtnrServiceTags: []string{"unitTest_ctnr"},
                     },
                     LogLevel: LogConfig {
                            Verbose: 3,
                            LogDir: "/test/logdir",
                     },
              }
       })
	   
	   AfterEach(func(){
 
       })
 
       Describe("Flag set test",func(){
              JustBeforeEach(func(){
                     create_configfile(g_testcfg,default_cfgfile)
              })
              AfterEach(func(){
                     os.Remove(default_cfgfile)
              })
 
              It("Test",func(){
                     By("Step 1,FlagInit and ParseConfig",func(){
                            FlagInit()
                            // set some value for test
                            flag.Set("interval","15")
                            flag.Parse()
                            err := ParseConfig(default_cfgfile)
                            Expect(err).NotTo(HaveOccurred())
                            conf := Config()
                            Expect(conf.Interval).To(Equal(15))
                     })
					 
					 By("Step 2,Flag set repeat",func(){
                            FlagInit()
                            // set some value for test
                            flag.Set("interval","10")
                            flag.Parse()
                            err := ParseConfig(default_cfgfile)
                            Expect(err).NotTo(HaveOccurred())
                            conf := Config()
                            Expect(conf.Interval).To(Equal(10))
                     })
              })
       })
 
       Describe("ParseConfig",func(){
              JustBeforeEach(func(){
                     create_configfile(g_testcfg,default_cfgfile)
              })
              AfterEach(func(){
                     os.Remove(default_cfgfile)
              })
			  
			  It("when input param error",func(){
                     By("Step 1,cfg is empty",func(){
                            tmpcfg := ""
                            err := ParseConfig(tmpcfg)
                            Expect(err).To(HaveOccurred())
                     })
                     By("Step 2,cfg is not exist",func(){
                            tmpcfg := "notexistcfg"
                            err := ParseConfig(tmpcfg)
                            Expect(err).To(HaveOccurred())
                     })
                     By("Step 3,cfg unmarshal failed",func(){
                            type errcfg struct {
                                   LocalIp    int `json:"localIp"`
                            }
 
                            tmpconf := errcfg {
                                   LocalIp: 3,
                            }
							
							create_configfile(tmpconf,default_cfgfile)
                            err := ParseConfig(default_cfgfile)                          
                            Expect(err).To(HaveOccurred())
                     })
                     By("Step 4,local ip is empty",func(){
                            tmpconf := g_testcfg
                            tmpconf.LocalIP = ""
 
                            create_configfile(tmpconf,default_cfgfile)
                            err := ParseConfig(default_cfgfile)
                            Expect(err).NotTo(HaveOccurred())
                     })
              })
              It("when input normal data",func(){
                     By("Step 1,normal check ParseConfig",func(){
                            err := ParseConfig(default_cfgfile)
                            Expect(err).NotTo(HaveOccurred())
                     })
              })
       })
	   
	   Describe("CheckDefault",func () {
              JustBeforeEach(func(){
                     create_configfile(g_testcfg,default_cfgfile)
              })
              AfterEach(func(){
                     os.Remove(default_cfgfile)
              })
              It("When default value is invalid",func () {
                     flag.Set("interval","2")
                     flag.Set("maxdockerexeccount","399")
                     flag.Set("configcheckduration","4")
                     flag.Set("redis_maxidle","99")
                     flag.Set("redis_maxactive","599")
                     flag.Set("mysql_maxidle","19")
                     flag.Set("mysql_maxopen","19")
                     flag.Set("mysql_maxlifetime","299")
                     flag.Set("signmsg_interval","299")
                     flag.Set("health_timeout","9")
                     flag.Set("health_interval","4")
					 flag.Set("health_interval","4")
                     flag.Set("health_maxtries","9")
                     flag.Set("route_pushinterval","29")
                     flag.Set("consul_updateinterval","59")
                     flag.Parse()
 
                     err := ParseConfig(default_cfgfile)
                     Expect(err).NotTo(HaveOccurred())
                     c := Config()
 
                     c.CheckDefault()
                     c = Config()
                     Expect(c.Interval).To(Equal(3))
                     Expect(c.MaxDockerExecCount).To(Equal(400))
                     Expect(c.ConfigCheckDuration).To(Equal(5))
                     Expect(c.Redis.MaxIdle).To(Equal(100))
                     Expect(c.Redis.MaxActive).To(Equal(600))
                     Expect(c.DB.MaxIdle).To(Equal(20))
                     Expect(c.DB.MaxOpen).To(Equal(20))
                     Expect(c.DB.MaxLifetime).To(Equal(300))
					 Expect(c.SignMsg.Interval).To(Equal(int64(300)))
                     Expect(c.Health.TimeOut).To(Equal(10))
                     Expect(c.Health.Interval).To(Equal(5))
                     Expect(c.Health.MaxTries).To(Equal(10))
                     Expect(c.Route.PushInterval).To(Equal(30))
                     Expect(c.ConsulServer.UpdateCtnrInterval).To(Equal(60))       
              })           
       })
 
       Describe("DyncCfg",func () {
              JustBeforeEach(func(){
                     create_configfile(g_testcfg,default_cfgfile)
              })
              AfterEach(func(){
                     os.Remove(default_cfgfile)
              })
              It("Testcase",func(){
                     By("Step 1,cfg is empty",func(){
                            err := DyncCfg("")
                            Expect(err).To(HaveOccurred())
                     })
					 By("Step 2,cfg is not exist",func () {
                            err := DyncCfg("notexistcfg")
                            Expect(err).To(HaveOccurred())
                     })
                     By("Step 3,cfg change",func(){
                            tmpconf := g_testcfg
                            tmpconf.MAINT.Switch = true
                            tmpconf.LogLevel.Verbose = 3
                            create_configfile(tmpconf,default_cfgfile)
 
                            err := DyncCfg(default_cfgfile)
                            Expect(err).NotTo(HaveOccurred())
                            conf := Config()
                            Expect(conf.MAINT.Switch).To(Equal(true))
                            Expect(conf.LogLevel.Verbose).To(Equal(3))
                     })
              })
       })
	   
	   Describe("WatchFile",func(){
              JustBeforeEach(func(){
                     create_configfile(g_testcfg,default_cfgfile)
              })
              AfterEach(func(){
                     os.Remove(default_cfgfile)
              })
              It("Testcase",func(){
                     ParseConfig(default_cfgfile)
                     conf := Config()
                     Expect(conf.MAINT.Switch).To(Equal(false))
 
                     go WatchFile(default_cfgfile)
                     time.Sleep(time.Duration(1) * time.Second)
                     tmpconf := g_testcfg
                     tmpconf.MAINT.Switch = true
                     create_configfile(tmpconf,default_cfgfile)
                     //
                     time.Sleep(time.Duration(conf.ConfigCheckDuration) * time.Second)
 
                     conf = Config()
                     Expect(conf.MAINT.Switch).To(Equal(true))
              })
       })
	   
	   Describe("PrintlnAndReturnErr",func(){
              It("when errorstring is empty",func(){
                     var errorstring string
                     err := PrintlnAndReturnErr(errorstring)
                     Expect(err.Error()).To(Equal(errorstring))
              })
              It("When errorstring is normal",func(){
                     var errorstring string = "test error string"
                     err := PrintlnAndReturnErr(errorstring)
                     Expect(err.Error()).To(Equal(errorstring))
              })
       })
	   
	   Describe("NewNetClient",func () {
              JustBeforeEach(func(){
                     create_configfile(g_testcfg,default_cfgfile)
              })
			  
			  AfterEach(func(){
                     os.Remove(default_cfgfile)
              })
              It("When switch is false",func () {
                     g_testcfg.NetService.Switch = false
                     create_configfile(g_testcfg,default_cfgfile)
 
                     err := ParseConfig(default_cfgfile)
                     Expect(err).NotTo(HaveOccurred())
                     err = NewNetClient()
                     Expect(err).To(BeNil())
                     result := NetClient()
                     Expect(result).To(BeNil())
              })
              It("When switch is true",func () {
                     err := ParseConfig(default_cfgfile)
                     Expect(err).NotTo(HaveOccurred())
                     err = NewNetClient()
                     Expect(err).To(BeNil())
                     result := NetClient()
                     Expect(result).NotTo(BeNil())
              })
       })
})
