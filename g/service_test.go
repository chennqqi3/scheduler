package g_test
 
import (
       storageapp "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       ME "github-beta.huawei.com/hipaas/common/msgengine"
       docker "github.com/fsouza/go-dockerclient"
       TP "github-beta.huawei.com/hipaas/common/util/taskpool"
       "github.com/hashicorp/consul/api"
 
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
       . "github-beta.huawei.com/hipaas/scheduler/g"
 
       "time"
       "fmt"
       "sync"
       "encoding/json"
       "strings"
       "math/rand"
)

var _ = Describe("Containers Service suite test",func(){
       var (
              err error
              csd CtnrSvcDiscv
              app storageapp.App
              containers []realstate.Container
              birthday time.Time = time.Now()
              consulClient *api.Client
              oncego   sync.Once
       )
       const appPortType string = "monitor"
 
       consulGetServices := func() (map[string]*api.AgentService, error) {
              oncego.Do(func(){
                     var err error
 
                     consulcfg := api.Config{
                            Address:       Config().ConsulServer.ConsulAddress,
                            Scheme:        Config().ConsulServer.ConsulScheme,
                            Datacenter:    Config().ConsulServer.ConsulDatacenter,
                     }
                     consulClient,err = api.NewClient(&consulcfg)
					 if nil != err {
                            fmt.Printf("New consul client failed,Error: %v\n",err)
                     }
              })
 
              agent := consulClient.Agent()
              return agent.Services()        
       }
      
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
 
       BeforeEach(func(){
              app = storageapp.App{
                     Name:        randomStr("hcytappn",10),
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
                            ID: randomStr("hcyctnido",14),
                            // IP: strings.Split(Config().ConsulServer.ConsulAddress,":")[0],
							IP: "127.0.0.1",
                            AppName: app.Name,
                            Status: "Up",
                            AppType: storageapp.AppTypeLRC,
                     },
                     {
                            ID: randomStr("ctnidhcy",14),
                            // IP: strings.Split(Config().ConsulServer.ConsulAddress,":")[0],
                            IP: "127.0.0.1",
                            AppName: app.Name,
                            Status: "Up",
                            AppType: storageapp.AppTypeSRC,
                     },
              }
       })
       AfterEach(func(){
 
       })
	   defer GinkgoRecover()
 
       It("ParseConfig, then Start CtnrServiceDiscover",func(){
              // g parse cfg
              FlagInit()
              err = ParseConfig("../cfg.example.json")
              if err != nil {
                     fmt.Printf("Error ParseConfig failed,%v\n",err)
                     Fail("ParseConfig failed")
              }
 
              // started container service
              pool, poolerr := TP.NewTaskPool(100)
              if nil != poolerr {
                     fmt.Printf("Create task pool failed,Error: %v\n",poolerr)
                     Fail("Create task pool failed,Error: "+poolerr.Error())
              }
              csd = CtnrSvcDiscv{
                     Task:           pool,
                     PortType:       appPortType,
                     Prefix:         Config().ConsulService.CtnrServicePrefix,
					 RegEventChan:   make(chan []byte, 300),
                     DeregEventChan: make(chan []byte, 300),
              }
              go csd.WaitEvent()
       })
 
       Context("Test Containers service write data to consul when recive event",func(){
              JustBeforeEach(func(){
                     for k,_ := range containers {
                            containers[k].IP = strings.Split(Config().ConsulServer.ConsulAddress,":")[0]
                     }
              })
 
              AfterEach(func(){
                     // release data
              })
 
              It("Send Event and checkout consul data",func(){
                     By("Add Containers to consul",func(){
                            // set app infos
                            app.Status = "Started"
                            portinfos := []realstate.PortInfo{
                                   {
										Portname: "monitor",
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
 
                            // make msg infos
                            msg := ME.AppStatusToStartedData{
                                   App: app,
                                   Containers: containers,
								   Birthday: birthday,
                                   Message: "test",
                            }
                            // send App started event                        
                            dat, err := json.Marshal(msg)
                            Expect(err).NotTo(HaveOccurred())
                            csd.RegEventChan <- dat
                            time.Sleep(time.Second * waitTime)
                            // check consul data
                            results,err := consulGetServices()
                            Expect(err).NotTo(HaveOccurred())
                            tmp_desired := 0
							for _,agent := range results {
                                   if strings.Contains(agent.Service,app.Name) {
                                          tmp_desired  = 1
                                   }
                            }
                            Expect(tmp_desired).To(Equal(len(containers)))
                     })           
                     By("Delete Containers to consul",func(){
                            // send App update event
                            for _,container := range containers {
                                   msg := ME.DeleteOneContainerForAnyReasonData {
                                          Birthday: birthday,
                                          Container: container,
                                          Message: "test",
                                   }
                                   dat,err := json.Marshal(msg)
								   Expect(err).NotTo(HaveOccurred())
                                   csd.DeregEventChan <- dat                           
                            }
 
                            time.Sleep(time.Second * waitTime)
                            // check consul data
                            results,err := consulGetServices()
                            Expect(err).NotTo(HaveOccurred())
                            tmp_desired := 0
                            for _,agent := range results {
                                   if strings.Contains(agent.Service,app.Name) {
                                          tmp_desired  = 1
                                          if tmp_desired > 0 {
                                                 break
                                          }
                                   }
                            }
                            Expect(tmp_desired).To(Equal(0))
                     })
              })
       })
})

