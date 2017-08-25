package g_test
 
import (
       storageapp "github-beta.huawei.com/hipaas/common/storage/app"
       ctnrpub "github-beta.huawei.com/hipaas/common/ctnrpublish"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       ME "github-beta.huawei.com/hipaas/common/msgengine"
      
       "github.com/garyburd/redigo/redis"
       "github-beta.huawei.com/hipaas/common/crypto/aes"
       docker "github.com/fsouza/go-dockerclient"
 
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
       . "github-beta.huawei.com/hipaas/scheduler/g"
 
       "time"
       "fmt"
       "math/rand"  
)
 
var _ = Describe("Ctnrpublish suite test",func(){
       var (
              err error
              app storageapp.App
              containers []realstate.Container
              birthday time.Time = time.Now()
       )

newRedisPool := func(server, password string) *redis.Pool {
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
	   
	   consulGetContainers := func(appName string) (*[]realstate.Container,error) {
              cfg := ctnrpub.ConsulPubConfig{
                     Address:       Config().ConsulServer.ConsulAddress,
                     Scheme:        Config().ConsulServer.ConsulScheme,
                     Datacenter:    Config().ConsulServer.ConsulDatacenter,
                     Token:         Config().ConsulServer.CtnrToken,
              }
              ctnrPublisher, err := ctnrpub.NewCtnrPubDrv(cfg);
              if nil != err {
                     return &[]realstate.Container{},err
              }
 
              ctnrs,err := ctnrPublisher.GetAppCtnrs(appName)
              if nil != err {
                     return &[]realstate.Container{},err
              }
 
              return ctnrs,nil             
       }
	   
	   randomStr := func(prefix string,ls int) string{
              str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
              bytes := []byte(str)
              result := []byte{}
              r := rand.New(rand.NewSource(time.Now().UnixNano()))
              for i :=0;i < ls; i   {
                     result = append(result,bytes[r.Intn(len(bytes))])
              }
              return prefix string(result)
       }
 
       BeforeEach(func(){
              app = storageapp.App{
                     Name:        randomStr("ctnrpubts",10),
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
                     Status:   storageapp.AppStatusStartSuccess,
                     Region:   "regiontest",
                     VMType:   "container",
                     Hostnames:    []storageapp.Hostname{storageapp.Hostname{Hostname: "huangcuiyang", Subdomain: "huawei.com", Status: "1"}},
              }
              containers = []realstate.Container{
                     {
                            ID: randomStr("ctnrpbID",14),
                            IP: "10.62.136.67",
                            AppName: app.Name,
                            Status: realstate.ContainerStatusUp,
                     },
              }
       })
	   
	   AfterEach(func(){
 
       })
 
       defer GinkgoRecover()
 
       It("ParseConfig ,then StartME and StartCtnrPublisher",func(){
              // g parse cfg
              // need only once run
              FlagInit()
              err = ParseConfig("../cfg.example.json")
              if err != nil {
                     fmt.Printf("Error ParseConfig failed,%v\n",err)
                     Fail("ParseConfig failed")
              }
              // Started ME
              redisPool := newRedisPool(Config().Redis.Dsn, Config().Redis.PassWord)
              switchChan := make(chan bool)
              err = ME.StartMsgEngine(redisPool, &switchChan, "metag")
              if err != nil {
                     fmt.Printf("Error Start MsgEngine,%v\n",err)
                     Fail("Started MsgEngine failed")
              }
			  switchChan <- false
              // started CtnrPublisher
              err = StartCtnrPublisher()
              if err != nil {
                     Fail("Started CtnrPublish Failed")
              }
       })
 
       Context("Test Ctnrpublish write data to consul when recive event from ME",func(){
              JustBeforeEach(func(){
 
              })
 
              AfterEach(func(){
                     // release data
              })
 
              It("Add and Del actions",func(){
                     By("Add Containers to consul",func(){
                            // set app infos
                            portinfos := []realstate.PortInfo{
                                   {
                                          Portname: realstate.PublicPortName,
                                          Ports: []docker.PortBinding{
                                                 {		HostIP: "127.0.0.1",
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
 
                            // make msg infos
                            msg := ME.AppStatusToStartedData{
                                   App: app,
                                   Containers: containers,
                                   Birthday: birthday,
                                   Message: "test",
                            }
                            // send App started event
							ME.NewEventReporter(ME.AppStatusToStarted,msg)
                            time.Sleep(time.Second * waitTime)
                            // check consul data
                            results,err := consulGetContainers(app.Name)
                            Expect(err).NotTo(HaveOccurred())
                            Expect(len(*results)).To(Equal(len(containers)))
                            // Expect(results).To(Equal(containers))
                            for _,result := range *results {
                                   flag := true
                                   for _,container := range containers {
                                          if result.ID == container.ID {
                                                 flag = false
                                                 Expect(result.IP).To(Equal(container.IP))
												 Expect(result.AppName).To(Equal(container.AppName))
                                                 // Expect(result.Status).To(Equal(container.Status))
                                                 Expect(result.PortInfos).To(Equal(container.PortInfos))
                                                 break
                                          }
                                   }
                                   if flag {
                                          Fail("Can not be here")
                                   }
                            }
                     })           
                     By("Delete Containers to consul",func(){
                            // send App update event
                            msg := ME.DeleteContainersBecauseTheAppItBelongsWasDeletedData {
								   AppName: app.Name,
								   Containers: containers,
                                   Message: "test",
                            }
                            ME.NewEventReporter(ME.DeleteContainersBecauseTheAppItBelongsWasDeleted,msg)
                            time.Sleep(time.Second * waitTime)
                            // check consul data
                            results,err := consulGetContainers(app.Name)
                            Expect(err).NotTo(HaveOccurred())
                            Expect(len(*results)).To(Equal(0))
                     })                  
              })
       })
})
