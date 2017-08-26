package executor_test
 
import (
       storageapp "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "github-beta.huawei.com/hipaas/common/redisoperate"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/common/storage/createstate"
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
       . "github-beta.huawei.com/hipaas/scheduler/executor"
       "fmt"
       "time"
       "math/rand"
       "net/rpc"
       "net"
       "sync"
)
 
// type DockerRuntimeProvider interface {
//    DockerExecAsync(exec string, arg interface{}, beforeExec func() error, afterExec func(error))
//    GetDockerLogs(appName string, ctnrID string) ([]*createstate.DockerLogs, error)
// }
const testcfg = "../cfg.example.json"
 
// for rpc test
type Execute struct {
}

func (executor *Execute) DockDel(req *realstate.Container, resp *createstate.CommonContainerExecResponse) error  {
       return nil
}

func (executor *Execute) NewDockRun(req *createstate.RunContainerOption, resp *createstate.RunContainerResponse) (err error) {
       resp.Container=&realstate.Container{
              ID: "UnitTestCTNRID203950687264334",
              IP: req.ActionOption.AgentIP,
              AppName: req.CreateOption.Name,
              Status: realstate.ContainerStatusUp,         
       }
       return nil
}
func (executor *Execute) DockerRestart(req *realstate.Container, resp *createstate.RunContainerResponse) error {
       resp.Container=&realstate.Container{
              ID: "UnitTestCTNRID203950687264334",
              IP: req.IP,
              AppName: req.AppName,
              Status: realstate.ContainerStatusUp,         
       }
       return nil
}

func (executor *Execute) DockerStop(req *realstate.Container, resp *createstate.RunContainerResponse) error {
       resp.Container=&realstate.Container{
              ID: "UnitTestCTNRID203950687264334",
              IP: req.IP,
              AppName: req.AppName,
              Status: realstate.ContainerStatusUp,         
       }
       return nil
}

func (executor *Execute) DockerLogs(req *realstate.Container, resp *string) error {
       tmpstr := "testdockerlogs"
       resp=&tmpstr
       return nil
}

func (executor *Execute) NodeTaskCreateCtnr(req *createstate.NodeTaskCreateCtnr, resp *createstate.RPCCommonRespone) error {
       return nil
}
 
var _ = Describe("dockerrun suite test",func(){
       var (
              dockerMG DockerRuntimeProvider
              app *storageapp.App
              deployinfo *node.DeployInfo
              ctnr *realstate.Container
			  syncWait time.Duration = 1*60
              checkinterval time.Duration = 1
              oncego sync.Once
       )
 
       randomstr := func(prefix string,lns int) string{
              str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
              bytes := []byte(str)
              result := []byte{}
              r := rand.New(rand.NewSource(time.Now().UnixNano()))
              for i :=0;i < lns; i   {
                     result = append(result,bytes[r.Intn(len(bytes))])
              }
              return prefix string(result)
       }
 
       defer GinkgoRecover()
       // parse cfg
       g.FlagInit()
       err := g.ParseConfig(testcfg)
       if err != nil {
              Fail("g.ParseConfig failed: " err.Error())
       }
       redisConnPool,err := redisoperate.InitRedisConnPool(g.Config().Redis)
       if err != nil {
              Fail("redisConnPool Init failed: " err.Error())
       }
       driver := "redis"
	   g.RealState,err = realstate.NewSafeRealState(driver, redisConnPool)
       if err != nil {
              Fail("RealState can not init!")
       }
 
       dockerMG = GetDockerManager()
 
       // set async time control
       SetDefaultEventuallyTimeout(time.Second * syncWait)
       SetDefaultEventuallyPollingInterval(time.Second * checkinterval)
 
       BeforeEach(func(){
              app = &storageapp.App {
                     Name:        randomstr("hcyapp",10),
                     CPU:         1,
                     Memory:      128,
                     Instance: 1,
					 Image: storageapp.Image{
                            DockerImageURL:    "9.91.17.17:5000/wujianln/dls",
                            DockerLoginServer: "",
                            DockerUser:        "",
                            DockerPassword:    "",
                            DockerEmail:       "wujianlin1@huawei.com",
                     },
                     Recovery: 0,
                     Status:   storageapp.AppStatusStartSuccess,
                     Region:   "fg",
                     VMType:   "df",
					 Hostnames:    []storageapp.Hostname{storageapp.Hostname{Hostname: "zhangmanjuan", Subdomain: "huawei.com", Status: "1"}},
                     Mount:    []storageapp.Mount{
                            {
                                   HPath: "/test/dir",
								   CPath: "/testdir",
                            },
                     },
              }
              deployinfo = &node.DeployInfo{
                     AgentIP:     "127.0.0.1",
                     Region:      "region1",
                     Hostname:    "huangcuiyang",
                     Ctnrname:     randomstr("hcyctnr",10),
                     ContainerIP: "10.63.123.62",
                     Gateway:     "10.255.255.255",
                     Mask:        "255.0.0.0",
                     Recovery:    "1",
              }
              ctnr = &realstate.Container {
                     ID: randomstr("hcycid",14),
                     IP: "127.0.0.1",
                     AppName: app.Name,
					 Status: realstate.ContainerStatusUp,
              }
              // start rpc service for test
              oncego.Do(func () {
                     go func () {
                            addr := fmt.Sprintf("%s:%d", deployinfo.AgentIP, g.Config().AgentRPCPort)
                            dockerExec := &Execute{}
 
                            rpc.Register(dockerExec)
                            l,err := net.Listen("tcp",addr)
                            if err != nil {
                                   return
                            }
                            for {
                                   conn,err := l.Accept()
                                   if err != nil {
                                          continue
                                   }
                                   go rpc.ServeConn(conn)
                            }
                     }()
              })
       })
       AfterEach(func(){
 
       })
	   Context("DockerExecAsync test",func(){
              var (
                     beforeExec func() error
                     afterExec func(error)
                     execStr string
                     arg interface{}
              )
              type myTestST struct {
                     Name string
                     ID   int
              }
 
              JustBeforeEach(func(){
 
              })
 
              It("exec unsupport",func(){
                     beforeExec = func() error {
                            Fail("Can not in here")
                            return nil
                     }
                     afterExec = func(error) {
                            Fail("Can not in here")
                     }
                     execStr = "unsupport"
 
                     dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
              })
			  It("exec Create",func(){
                     By("arg is not DockerInstance type",func(){
                            beforeExec = func () error {
                                   Fail("Can not in here")
                                   return nil
                            }
                            afterExec = func (err error) {
                                   Fail("Can not in here")
                            }
                            execStr = Create
                            arg = &myTestST{
                                   Name: "test",
                                   ID: 1,
                            }
                            dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                     })
                     By("beforeExec error",func(){
                            c := make(chan bool)
 
                            beforeExec = func() error {
                                   c <- true
								   return fmt.Errorf("exec failed")
                            }
                            afterExec = func(err error) {
                                   Fail("Can not in here")
                            }
                            execStr = Create
                            arg = &DockerInstance{
                                   App: app,
                                   DeployInfo: deployinfo,
                            }
 
                            dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                            Eventually(c).Should(Receive(Equal(true)))
                     })
                     By("beforeExec ok",func(){
                            c := make(chan bool,2)
 
                            beforeExec = func() error {
                                   c <- true
                                   return nil
                            }
							afterExec = func(err error) {
                                   c <- true
                            }
                            execStr = Create
                            arg = &DockerInstance{
                                   App: app,
                                   DeployInfo: deployinfo,
                            }
 
                            dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                            Eventually(c).Should(HaveLen(2))
                     })
              })
              It("exec Drop",func(){
                     By("arg is not realstate.Container type",func(){
                            beforeExec = func () error {
                                   Fail("Can not in here")
                                   return nil
                            }
                            afterExec = func (err error) {
                                   Fail("Can not in here")
                            }
							execStr = Drop
                            arg = &myTestST{
                                   Name: "test",
                                   ID: 1,
                            }
                            dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                     })                  
                     By("beforeExec error",func(){
                            c := make(chan bool,2)
 
                            beforeExec = func() error {
                                   c <- true
                                   return fmt.Errorf("exec failed")
                            }
                            afterExec = func(err error) {
                                   c <- true
                            }
                            execStr = Drop
                            arg = ctnr
							dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                            Eventually(c).Should(HaveLen(1))
                     })
                     By("beforeExec ok",func(){
                            c := make(chan bool,2)
                            beforeExec = func() error {
                                   c <- true
                                   return nil
                            }
                            afterExec = func(err error) {
                                   c <- true
                            }
                            execStr = Drop
                            arg = ctnr
							dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                            Eventually(c).Should(HaveLen(2))
                     })
              })
              It("exec Stop",func(){
                     By("arg is not realstate.Container type",func(){
                            beforeExec = func () error {
                                   Fail("Can not in here")
                                   return nil
                            }
                            afterExec = func (err error) {
                                   Fail("Can not in here")
                            }
                            execStr = Stop
                            arg = &myTestST{
                                   Name: "test",
                                   ID: 1,
                            }
							dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                     })                         
                     By("beforeExec error",func(){
                            c := make(chan bool,2)
 
                            beforeExec = func() error {
                                   c <- true
                                   return fmt.Errorf("exec failed")
                            }
                            afterExec = func(err error) {
                                   c <- true
                            }
                            execStr = Stop
                            arg = ctnr
							dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                            Eventually(c).Should(HaveLen(1))
                     })
                     By("beforeExec ok",func(){
                            c := make(chan bool,2)
 
                            beforeExec = func() error {
                                   c <- true
                                   return nil
                            }
                            afterExec = func(err error) {
                                   c <- true
                            }
                            execStr = Stop
                            arg = ctnr
							dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                            Eventually(c).Should(HaveLen(2))
                     })
              })
              It("exec Restart",func(){
                     By("arg is not realstate.Container type",func(){
                            beforeExec = func () error {
                                   Fail("Can not in here")
                                   return nil
                            }
                            afterExec = func (err error) {
                                   Fail("Can not in here")
                            }
                            execStr = Restart
                            arg = &myTestST{
                                   Name: "test",
                                   ID: 1,
                            }
							dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                     })                         
                     By("beforeExec error",func(){
                            c := make(chan bool,2)
 
                            beforeExec = func() error {
                                   c <- true
                                   return fmt.Errorf("exec failed")
                            }
                            afterExec = func(err error) {
                                   c <- true
                            }
                            execStr = Restart
                            arg = ctnr
							dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                            Eventually(c).Should(HaveLen(1))
                     })
                     By("beforeExec ok",func(){
                            c := make(chan bool,2)
 
                            beforeExec = func() error {
                                   c <- true
                                   return nil
                            }
                            afterExec = func(err error) {
                                   c <- true
                            }
                            execStr = Restart
                            arg = ctnr
							dockerMG.DockerExecAsync(execStr,arg,beforeExec,afterExec)
                            Eventually(c).Should(HaveLen(2))
                     })
              })
       })
       Context("GetDockerLogs test",func(){
              AfterEach(func(){
                     g.RealState.DeleteSafeApp(app.Name)
              })
 
              It("when redis not exist this app",func(){
                     ret_dockerlogs,err := dockerMG.GetDockerLogs(app.Name,ctnr.ID)
                     Expect(err).To(HaveOccurred(),"Can't find any container by this app name: " app.Name)
                     Expect(ret_dockerlogs).To(BeNil())
              })
			  It("when redis exist this app",func(){
                     By("add app to redis",func(){
                            g.RealState.UpdateContainer(ctnr)
                     })
                     By("get DockerLogs",func(){
                            ret_dockerlogs,err := dockerMG.GetDockerLogs(app.Name,ctnr.ID)
                            Expect(err).To(BeNil())
                            Expect(len(ret_dockerlogs)).To(Equal(1))
                     })
              })
       })
       Context("NodeTaskCreateCtnr",func(){
              It("Test",func(){
                     nodeip := "127.0.0.1"
                     appsDeployInfos := []*node.NodeAppDeployInfo{
                            {
                                   App: app,
                                   DeployInfos: []*node.DeployInfo{
                                          deployinfo,
                                   },
                            },
                     }
					 err := dockerMG.NodeTaskCreateCtnr(nodeip,appsDeployInfos)
                     Expect(err).To(BeNil())
              })
       })
})
