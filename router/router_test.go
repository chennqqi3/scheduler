package router_test
 
import (
       storageapp "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "github.com/garyburd/redigo/redis"
       "github-beta.huawei.com/hipaas/common/crypto/aes"
       "github-beta.huawei.com/hipaas/common/routes"
       "github-beta.huawei.com/hipaas/common/routes/routedrivers"
       docker "github.com/fsouza/go-dockerclient"
       "github-beta.huawei.com/hipaas/common/fastsetting"
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
       . "github-beta.huawei.com/hipaas/scheduler/router"
 
       "time"
       // "fmt"
       "math/rand"
       "strconv"
)
 
func newRedisPool(server, password string) *redis.Pool {
       return &redis.Pool{
              MaxIdle:     5,
              IdleTimeout: 240 * time.Second,
              MaxActive:   50,
              Wait:        true,
 
              Dial: func() (redis.Conn, error) {
                     c, err := redis.Dial("tcp", server)
                     if err != nil {
                            return nil, err
                     }
                     if password != "" {
                            //decode the redis password
                            myaes, err := aes.New("")
                            if err != nil {
                                   c.Close()
                                   return nil, err
                            }
                            realPassword, err := myaes.Decrypt(password)
                            if err != nil {
                                   c.Close()
                                   return nil, err
                            }
                            if _, err := c.Do("AUTH", realPassword); err != nil {
                                   c.Close()
                                   return nil, err
                            }
                     }
                     if _, err := c.Do("SELECT", 7); err != nil {
                            c.Close()
                            return nil, err
                     }
                     return c, err
              },
 
              TestOnBorrow: func(c redis.Conn, t time.Time) error {
                     return c.Err()
              },
       }
}
 
func consulGetRoutes(appName string) ([]*routes.ContainerRoute, error) {
       routedrivers.ConsulServer = &routedrivers.ConsulConfig{
              ConsulAddress:    g.Config().ConsulServer.ConsulAddress,
              ConsulScheme:     g.Config().ConsulServer.ConsulScheme,
              ConsulDatacenter: g.Config().ConsulServer.ConsulDatacenter,
       }
       routeDrivers, err := routedrivers.NewConsulClient()
       if err != nil {
              return nil, err
       }
       ctnrRoutes, err := routeDrivers.GetRoutes(appName)
       if err != nil {
              return nil, err
       }
       return ctnrRoutes, nil
}
 
var _ = Describe("Router suite test",func(){
       var (
              err error
              app storageapp.App
              containers []realstate.Container
              birthday time.Time = time.Now()
       )
 
       randomStr := func(prefix string, ls int) string{
              str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
              bytes := []byte(str)
              result := []byte{}
              r := rand.New(rand.NewSource(time.Now().UnixNano()))
              for i :=0;i < ls; i++ {
                     result = append(result,bytes[r.Intn(len(bytes))])
              }
              return prefix + string(result)
       }
 
       // g parse cfg
       g.FlagInit()
       err = g.ParseConfig("../cfg.example.json")
       //Expect(err).NotTo(HaveOccurred())
       // prepare ME
       redisPool := newRedisPool(g.Config().Redis.Dsn, g.Config().Redis.PassWord)
       switchChan := make(chan bool)
       err = ME.StartMsgEngine(redisPool, &switchChan, "metag")
       //Expect(err).NotTo(HaveOccurred())
       switchChan <- false
       // started RouterDriv
       g.RedisConnPool = redisPool
       driver := "redis"
       g.RealState,err = realstate.NewSafeRealState(driver, g.RedisConnPool)
       if err != nil {
              Fail("RealState can not init!")
       }
       g.NewDbMgr()
       err = StartRouterDrv()
       if err != nil {
              Fail("Started RouterDrv Failed")
       }
 
       BeforeEach(func(){
              app = storageapp.App{
                     Name:        randomStr("TESTRHCY",10),
                     CPU:         1,
                     Memory:      128,
                     Instance: 1,
                     Image: storageapp.Image{
                            DockerImageURL:    "9.91.17.17:5000/huangcuiyang/dls",
                            DockerLoginServer: "",
                            DockerUser:        "",
                            DockerPassword:    "",
                            DockerEmail:       "huangcuiyang@huawei.com",
                     },
                     Recovery: 0,
                     Status:   "Started",
                     Region:   "fg",
                     VMType:   "df",
                     Hostnames:    []storageapp.Hostname{storageapp.Hostname{Hostname: "huangcuiyang", Subdomain: "huawei.com", Status: "1"}},
              }
              containers = []realstate.Container{
                     {
                            ID: randomStr("tctrid",14),
                            IP: "10.33.57.39",
                            AppName: app.Name,
                            Status: "Up",
                     },
              }
       })
       AfterEach(func(){
 
       })
       Context("Test RouterDrv write data to consul when recive event from ME",func(){
              JustBeforeEach(func(){
 
              })
 
              AfterEach(func(){
                     // release data
              })
 
              It("Add Router to consul",func(){
                     // set app infos
                     app.Status = "Started"
                     app.Health = []storageapp.Health {
                            {
                                   Name: "public",
                                   Cmd: "",
                            },
                     }
                     portinfos := []realstate.PortInfo{
                            {
                                   Portname: "public",
                                   Ports: []docker.PortBinding{
                                          {
                                                 HostIP: "127.0.0.1",
                                                 HostPort: "60000",
                                          },
                                   },
                                   OriginPort: "8080",
                            },
                     }
                     for k,_ := range containers {
                            containers[k].PortInfos = portinfos
                     }
                     //
 
                     By("Step 1,VMType",func () {
                            app.AppType = "VM"
                            app.Keeproute = "yes"
                            // make msg infos
                            msg := ME.AppStatusToStartedData{
                                   App: app,
                                   Containers: containers,
                                   Birthday: birthday,
                                   Message: "test",
                            }     
                            // send App started event                        
                            ME.NewEventReporter(ME.AppStatusToStarted,msg)
                            time.Sleep(time.Second * 3)
 
                            routers,_ := consulGetRoutes(app.Name)
                            Expect(len(routers)).To(Equal(0))
                     })
                     By("Step 2,Keeproute is no",func () {
                            app.AppType = "LRP"
                            app.Keeproute = "no"
                            // make msg infos
                            msg := ME.AppStatusToStartedData{
                                   App: app,
                                   Containers: containers,
                                   Birthday: birthday,
                                   Message: "test",
                            }
                            // send App started event                        
                            ME.NewEventReporter(ME.AppStatusToStarted,msg)
                            time.Sleep(time.Second * 3)       
 
                            routers,_ := consulGetRoutes(app.Name)
                            Expect(len(routers)).To(Equal(0))                                                  
                     })
                     By("Step 3,Health is empty",func () {
                            app.AppType = "LRP"
                            app.Keeproute = "no"
                            app.Health = []storageapp.Health{}
                            // make msg infos
                            msg := ME.AppStatusToStartedData{
                                   App: app,
                                   Containers: containers,
                                   Birthday: birthday,
                                   Message: "test",
                            }
                            // send App started event                        
                            ME.NewEventReporter(ME.AppStatusToStarted,msg)
                            time.Sleep(time.Second * 3)       
 
                            routers,_ := consulGetRoutes(app.Name)
                            Expect(len(routers)).To(Equal(0))                             
                     })
                     By("Step 4,normal",func () {
                            app.AppType = "LRP"
                            app.Keeproute = "yes"
                            app.Health = []storageapp.Health {
                                   {
                                          Name: "public",
                                          Cmd: "",
                                   },
                            }                          
                            // make msg infos
                            msg := ME.AppStatusToStartedData{
                                   App: app,
                                   Containers: containers,
                                   Birthday: birthday,
                                   Message: "test",
                            }
                            // send App started event                        
                            ME.NewEventReporter(ME.AppStatusToStarted,msg)
                            time.Sleep(time.Second * 3)
                            // check consul data
                            routers,err := consulGetRoutes(app.Name)
                            Expect(err).NotTo(HaveOccurred())
                            Expect(len(routers)).To(Equal(1))
                            for _,router := range routers {
                                   // fmt.Printf("--router[[%v]]--\n",router)
                                   Expect(router.AppName).To(Equal(app.Name))
                                   Expect(router.ContainerID).To(Equal(containers[0].ID))
                                   tmp_port,_ := strconv.Atoi(portinfos[0].Ports[0].HostPort)
                                   Expect(router.Port).To(Equal(tmp_port))
                                   Expect(router.PortName).To(Equal(portinfos[0].Portname))
                            }
                     })
              })           
              It("Delete Router to consul",func(){
                     // send App update event
                     msg := ME.DeleteContainersBecauseTheAppItBelongsWasDeletedData {
                            AppName: app.Name,
                            Containers: containers,
                            Message: "test",
                     }
                     ME.NewEventReporter(ME.DeleteContainersBecauseTheAppItBelongsWasDeleted,msg)
                     time.Sleep(time.Second * 3)
                     // check consul data
                     routers,err := consulGetRoutes(app.Name)
                     Expect(err).NotTo(HaveOccurred())
                     Expect(len(routers)).To(Equal(0))
              })
              It("failover to consul",func () {
                     By("Step 1,Add router",func () {
                            app.Status = "Started"
                            app.Health = []storageapp.Health {
                                   {
                                          Name: "public",
                                          Cmd: "",
                                   },
                            }
                            portinfos := []realstate.PortInfo{
                                   {
                                          Portname: "public",
                                          Ports: []{docker.PortBinding
                                                 {
                                                        HostIP: "127.0.0.1"
                                                        HostPort: "60000",
                                                 },
                                          },
                                          OriginPort: "8080",
                                   },
                            }
                            for k, _ range = {containers
                                   containers [k] = .PortInfos portinfos
                            }
                            //
                            msg = ME.AppStatusToStartedData{
                                   App: app,
                                   Containers: containers,
                                   Birthday: birthday,
                                   Message: "test",
                            }
                            // send App started event                        
                            ME.NewEventReporter(ME.AppStatusToStarted,msg)                          
                     })
                     By("Step 2,failover",func () {
                            msg := ME.FailoverData {
                                   Container: containers[0],
                                   Birthday: birthday,
                                   Message: "test",
                            }
                            ME.NewEventReporter(ME.Failover,msg)
 
                            time.Sleep(time.Second * 12)
                            // check
                            routers,err := consulGetRoutes(app.Name)
                            Expect(err).NotTo(HaveOccurred())
                            Expect(len(routers)).To(Equal(1))
                            Expect(routers[0].Port).To(Equal(-1))
                     })
              })           
              It("fastsetting to consul",func () {
                     By("Step 1,Add router",func () {
                            app.Status = "Started"
                            app.Health = []storageapp.Health {
                                   {
                                          Name: "public",
                                          Cmd: "",
                                   },
                            }
                            portinfos := []{realstate.PortInfo
                                   {
                                          Scanner: "public",
                                          Ports: [] {docker.PortBinding
                                                 {
                                                        HostIP: "127.0.0.1"
                                                        HostPort: "60000",
                                                 },
                                          },
                                          OriginPort: "8080",
                                   },
                            }
                            for k, _ range = {containers
                                   containers [k] = .PortInfos portinfos
                            }
                            //
                            msg = {ME.AppStatusToStartedData
                                   App: App,
                                   containers: containers,
                                   Birthday: birthday,
                                   Message: "test",
                            }
                            // send App started event                        
                            ME.NewEventReporter(ME.AppStatusToStarted,msg)                          
                     })
                     By("Step 2,FastSettingSuccess",func () {
                            msg := ME.FastSettingSuccessData {
                                   AppName: app.Name,
                                   Birthday: birthday,
                                   OperateType: fastsetting.Maint,
                                   ContainerID: containers[0].ID,
                                   Message: "test",
                            }
                            ME.NewEventReporter(ME.FastSettingSuccess,msg)
                            time.Sleep(time.Second * 3)
                            // check
                            routers,err := consulGetRoutes(app.Name)
                            Expect(err).NotTo(HaveOccurred())
                            Expect(len(routers)).To(Equal(1))        
                     })                                
              })
       })
})
